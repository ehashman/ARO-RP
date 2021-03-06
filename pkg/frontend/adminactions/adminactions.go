package adminactions

// Copyright (c) Microsoft Corporation.
// Licensed under the Apache License 2.0.

import (
	"context"
	"net/http"

	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"

	"github.com/Azure/ARO-RP/pkg/api"
	"github.com/Azure/ARO-RP/pkg/env"
	"github.com/Azure/ARO-RP/pkg/util/azureclient/mgmt/compute"
	"github.com/Azure/ARO-RP/pkg/util/azureclient/mgmt/features"
	"github.com/Azure/ARO-RP/pkg/util/azureclient/mgmt/network"
	"github.com/Azure/ARO-RP/pkg/util/azureclient/mgmt/storage"
	"github.com/Azure/ARO-RP/pkg/util/dynamichelper"
	"github.com/Azure/ARO-RP/pkg/util/restconfig"
	"github.com/Azure/ARO-RP/pkg/util/stringutils"
)

// Interface for adminactions
type Interface interface {
	K8sGet(ctx context.Context, groupKind, namespace, name string) ([]byte, error)
	K8sList(ctx context.Context, groupKind, namespace string) ([]byte, error)
	K8sCreateOrUpdate(ctx context.Context, obj *unstructured.Unstructured) error
	K8sDelete(ctx context.Context, groupKind, namespace, name string) error
	ResourcesList(ctx context.Context) ([]byte, error)
	Upgrade(ctx context.Context, upgradeY bool) error
	VMRedeployAndWait(ctx context.Context, vmName string) error
	VMSerialConsole(ctx context.Context, w http.ResponseWriter,
		log *logrus.Entry, vmName string) error
}

type adminactions struct {
	log *logrus.Entry
	env env.Interface
	oc  *api.OpenShiftCluster
	dh  dynamichelper.Interface

	kubernetescli kubernetes.Interface
	configcli     configclient.Interface

	resources       features.ResourcesClient
	virtualMachines compute.VirtualMachinesClient
	virtualNetworks network.VirtualNetworksClient
	routeTables     network.RouteTablesClient
	storageAccounts storage.AccountsClient
}

// New returns an adminactions Interface
func New(log *logrus.Entry, env env.Interface, oc *api.OpenShiftCluster,
	subscriptionDoc *api.SubscriptionDocument) (Interface, error) {

	restConfig, err := restconfig.RestConfig(env, oc)
	if err != nil {
		return nil, err
	}

	dh, err := dynamichelper.New(log, restConfig)
	if err != nil {
		return nil, err
	}

	kubernetescli, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	configcli, err := configclient.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	fpAuth, err := env.FPAuthorizer(subscriptionDoc.Subscription.Properties.TenantID,
		env.Environment().ResourceManagerEndpoint)
	if err != nil {
		return nil, err
	}

	return &adminactions{
		log: log,
		env: env,
		oc:  oc,
		dh:  dh,

		kubernetescli: kubernetescli,
		configcli:     configcli,

		resources:       features.NewResourcesClient(env.Environment(), subscriptionDoc.ID, fpAuth),
		virtualMachines: compute.NewVirtualMachinesClient(env.Environment(), subscriptionDoc.ID, fpAuth),
		virtualNetworks: network.NewVirtualNetworksClient(env.Environment(), subscriptionDoc.ID, fpAuth),
		routeTables:     network.NewRouteTablesClient(env.Environment(), subscriptionDoc.ID, fpAuth),
		storageAccounts: storage.NewAccountsClient(env.Environment(), subscriptionDoc.ID, fpAuth),
	}, nil
}

func (a *adminactions) K8sGet(ctx context.Context, groupKind, namespace, name string) ([]byte, error) {
	un, err := a.dh.Get(ctx, groupKind, namespace, name)
	if err != nil {
		return nil, err
	}
	return un.MarshalJSON()
}

func (a *adminactions) K8sList(ctx context.Context, groupKind, namespace string) ([]byte, error) {
	ul, err := a.dh.List(ctx, groupKind, namespace)
	if err != nil {
		return nil, err
	}
	return ul.MarshalJSON()
}

func (a *adminactions) K8sCreateOrUpdate(ctx context.Context, obj *unstructured.Unstructured) error {
	return a.dh.CreateOrUpdate(ctx, obj)
}

func (a *adminactions) K8sDelete(ctx context.Context, groupKind, namespace, name string) error {
	return a.dh.Delete(ctx, groupKind, namespace, name)
}

func (a *adminactions) VMRedeployAndWait(ctx context.Context, vmName string) error {
	clusterRGName := stringutils.LastTokenByte(a.oc.Properties.ClusterProfile.ResourceGroupID, '/')
	return a.virtualMachines.RedeployAndWait(ctx, clusterRGName, vmName)
}
