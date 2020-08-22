package main

import (
	"os"

	controllergen "github.com/rancher/wrangler/pkg/controller-gen"
	"github.com/rancher/wrangler/pkg/controller-gen/args"
)

func main() {
	os.Unsetenv("GOPATH")
	controllergen.Run(args.Options{
		OutputPackage: "github.com/ibuildthecloud/k3s-operator/pkg/generated",
		Boilerplate:   "scripts/boilerplate.go.txt",
		Groups: map[string]args.Group{
			"k3s.ibtc.io": {
				Types: []interface{}{
					"./pkg/apis/k3s.ibtc.io/v1",
				},
				GenerateTypes: true,
			},
		},
	})
}
