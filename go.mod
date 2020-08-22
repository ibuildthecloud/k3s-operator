module github.com/ibuildthecloud/k3s-operator

go 1.14

replace k8s.io/client-go => k8s.io/client-go v0.18.0

require (
	github.com/golang/groupcache v0.0.0-20190702054246-869f871628b6 // indirect
	github.com/google/go-cmp v0.3.1 // indirect
	github.com/hashicorp/golang-lru v0.5.3 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.2 // indirect
	github.com/rancher/dynamiclistener v0.2.1-0.20200811000611-30cb223867a4
	github.com/rancher/lasso v0.0.0-20200820172840-0e4cc0ef5cb0
	github.com/rancher/wrangler v0.6.2-0.20200822010948-6d667521af49
	github.com/sirupsen/logrus v1.4.2
	github.com/urfave/cli v1.22.2
	golang.org/x/tools v0.0.0-20191017205301-920acffc3e65 // indirect
	google.golang.org/appengine v1.6.1 // indirect
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v0.18.8
)
