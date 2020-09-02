package crd

import (
	"context"
	"io"
	"os"
	"path/filepath"

	v1 "github.com/ibuildthecloud/k3s-operator/pkg/apis/k3s.ibtc.io/v1"
	"github.com/rancher/wrangler/pkg/crd"
	"github.com/rancher/wrangler/pkg/yaml"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

func WriteFile(filename string) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	return Print(f)
}

func Print(out io.Writer) error {
	obj, err := Objects()
	if err != nil {
		return err
	}

	data, err := yaml.Export(obj...)
	if err != nil {
		return err
	}

	_, err = out.Write(data)
	return err
}

func Objects() (result []runtime.Object, err error) {
	for _, crdDef := range List() {
		crd, err := crdDef.ToCustomResourceDefinition()
		if err != nil {
			return nil, err
		}
		result = append(result, &crd)
	}
	return
}

func List() []crd.CRD {
	return []crd.CRD{
		newCRD(&v1.K3s{}, func(c crd.CRD) crd.CRD {
			return c.
				WithColumn("Ready", ".status.ready").
				WithColumn("Kubeconfig", ".status.clientSecretName")
		}),
	}
}

func Create(ctx context.Context, cfg *rest.Config) error {
	factory, err := crd.NewFactoryFromClient(cfg)
	if err != nil {
		return err
	}

	return factory.BatchCreateCRDs(ctx, List()...).BatchWait()
}

func newCRD(obj interface{}, customize func(crd.CRD) crd.CRD) crd.CRD {
	crd := crd.CRD{
		GVK: schema.GroupVersionKind{
			Group:   "k3s.ibtc.io",
			Version: "v1",
		},
		Status:       true,
		SchemaObject: obj,
	}
	if customize != nil {
		crd = customize(crd)
	}
	return crd
}
