//go:generate go run pkg/codegen/cleanup/main.go
//go:generate go run pkg/codegen/main.go
//go:generate go run main.go --write-crds ./charts/k3s-operator/crds/crds.yaml

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ibuildthecloud/k3s-operator/pkg/controllers"
	"github.com/ibuildthecloud/k3s-operator/pkg/crd"
	"github.com/rancher/wrangler/pkg/kubeconfig"
	"github.com/rancher/wrangler/pkg/signals"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	_ "github.com/rancher/wrangler/pkg/generated/controllers/apiextensions.k8s.io/v1beta1"
)

var (
	Version    = "v0.0.0-dev"
	GitCommit  = "HEAD"
	KubeConfig string
	Context    string
	WriteCRDs  string
)

func main() {
	app := cli.NewApp()
	app.Name = "k3s-operator"
	app.Version = fmt.Sprintf("%s (%s)", Version, GitCommit)
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "kubeconfig",
			EnvVar:      "KUBECONFIG",
			Destination: &KubeConfig,
		},
		cli.StringFlag{
			Name:        "context",
			EnvVar:      "CONTEXT",
			Destination: &Context,
		},
		cli.StringFlag{
			Name:        "write-crds",
			Destination: &WriteCRDs,
		},
	}
	app.Action = run

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}

func run(c *cli.Context) error {
	if WriteCRDs != "" {
		logrus.Info("Writing CRDS to", WriteCRDs)
		return crd.WriteFile(WriteCRDs)
	}

	logrus.Info("Starting controller")
	ctx := signals.SetupSignalHandler(context.Background())
	kubeConfig, err := kubeconfig.GetNonInteractiveClientConfigWithContext(KubeConfig, Context).ClientConfig()
	if err != nil {
		return err
	}

	if err := controllers.Register(ctx, "", kubeConfig); err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}
