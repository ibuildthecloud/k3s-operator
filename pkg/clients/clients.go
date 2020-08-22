package clients

import (
	"context"

	"github.com/ibuildthecloud/k3s-operator/pkg/crd"
	"github.com/ibuildthecloud/k3s-operator/pkg/generated/controllers/k3s.ibtc.io"
	k3scontrollers "github.com/ibuildthecloud/k3s-operator/pkg/generated/controllers/k3s.ibtc.io/v1"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/generated/controllers/apps"
	appcontrollers "github.com/rancher/wrangler/pkg/generated/controllers/apps/v1"
	"github.com/rancher/wrangler/pkg/generated/controllers/core"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/generated/controllers/rbac"
	rbaccontrollers "github.com/rancher/wrangler/pkg/generated/controllers/rbac/v1"
	"github.com/rancher/wrangler/pkg/start"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Clients struct {
	k3scontrollers.Interface

	K8s        kubernetes.Interface
	Core       corecontrollers.Interface
	RBAC       rbaccontrollers.Interface
	Apps       appcontrollers.Interface
	Apply      apply.Apply
	RESTConfig *rest.Config
	starters   []start.Starter
}

func (a *Clients) Start(ctx context.Context) error {
	if err := crd.Create(ctx, a.RESTConfig); err != nil {
		return err
	}

	return start.All(ctx, 5, a.starters...)
}

func New(cfg *rest.Config) (*Clients, error) {
	core, err := core.NewFactoryFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	corev := core.Core().V1()

	apps, err := apps.NewFactoryFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	appsv := apps.Apps().V1()

	k3s, err := k3s.NewFactoryFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	k3sv := k3s.K3s().V1()

	rbac, err := rbac.NewFactoryFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	rbacv := rbac.Rbac().V1()

	apply, err := apply.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	apply = apply.WithSetOwnerReference(false, false)

	k8s, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &Clients{
		K8s:        k8s,
		Interface:  k3sv,
		Core:       corev,
		RBAC:       rbacv,
		Apps:       appsv,
		Apply:      apply,
		RESTConfig: cfg,
		starters: []start.Starter{
			core,
			k3s,
			rbac,
			apps,
		},
	}, nil
}
