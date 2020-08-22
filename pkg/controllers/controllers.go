package controllers

import (
	"context"

	"github.com/ibuildthecloud/k3s-operator/pkg/controllers/k3s"

	"github.com/ibuildthecloud/k3s-operator/pkg/clients"
	"github.com/rancher/wrangler/pkg/leader"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

func Register(ctx context.Context, systemNamespace string, cfg *rest.Config) error {
	clients, err := clients.New(cfg)
	if err != nil {
		return err
	}

	k3s.Register(ctx, clients)

	leader.RunOrDie(ctx, systemNamespace, "k3s-controller-lock", clients.K8s, func(ctx context.Context) {
		if err := clients.Start(ctx); err != nil {
			logrus.Fatal(err)
		}
	})

	return nil
}
