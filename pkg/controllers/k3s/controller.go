package k3s

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	v1 "github.com/ibuildthecloud/k3s-operator/pkg/apis/k3s.ibtc.io/v1"
	"github.com/ibuildthecloud/k3s-operator/pkg/clients"
	k3scontrollers "github.com/ibuildthecloud/k3s-operator/pkg/generated/controllers/k3s.ibtc.io/v1"
	"github.com/rancher/dynamiclistener/factory"
	tlsstorage "github.com/rancher/dynamiclistener/storage/kubernetes"
	corecontrollers "github.com/rancher/wrangler/pkg/generated/controllers/core/v1"
	"github.com/rancher/wrangler/pkg/kstatus"
	"github.com/rancher/wrangler/pkg/randomtoken"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

var (
	client = &http.Client{
		Transport: http.DefaultTransport.(*http.Transport).Clone(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	t = true
)

func init() {
	client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}
}

type handler struct {
	secrets      corecontrollers.SecretClient
	secretsCache corecontrollers.SecretCache
	k3sClient    k3scontrollers.K3sClient
}

func Register(
	ctx context.Context,
	clients *clients.Clients) {
	h := handler{
		secrets:      clients.Core.Secret(),
		secretsCache: clients.Core.Secret().Cache(),
		k3sClient:    clients.K3s(),
	}

	k3scontrollers.RegisterK3sGeneratingHandler(ctx,
		clients.K3s(),
		clients.Apply.WithDynamicLookup(),
		"Deployed",
		"k3s-deploy",
		h.OnK3sChange,
		nil)
	clients.K3s().OnChange(ctx, "k3s-status", h.checkReady)
}

func (c *handler) checkReady(key string, k3s *v1.K3s) (ret *v1.K3s, err error) {
	if k3s == nil || k3s.Status.CredentialSecretName == "" {
		return k3s, nil
	}

	defer func() {
		if ret == nil {
			return
		}

		if err != nil {
			err = fmt.Errorf("status check in-progress: %w", err)
		}

		if ret.Status.Ready != (err == nil) {
			ret = ret.DeepCopy()
			ret.Status.Ready = err == nil
			if ret.Status.Ready {
				kstatus.SetActive(&ret.Status)
			} else if err != nil {
				kstatus.SetTransitioning(&ret.Status, err.Error())
			}
			ret, err = c.k3sClient.UpdateStatus(ret)
		}
	}()

	secret, err := c.secretsCache.Get(k3s.Namespace, k3s.Status.CredentialSecretName)
	if err != nil {
		return k3s, err
	}

	cfg, err := clientcmd.RESTConfigFromKubeConfig(secret.Data["value"])
	if err != nil {
		return k3s, err
	}

	k8s, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return k3s, err
	}

	if _, err := k8s.Discovery().ServerVersion(); err != nil {
		return k3s, err
	}

	resp, err := client.Get(cfg.Host + "/ping")
	if err != nil {
		return k3s, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return k3s, fmt.Errorf("failed to GET %s, status code %d", cfg.Host, resp.StatusCode)
	}

	u, err := url.Parse(cfg.Host)
	if err != nil {
		return k3s, err
	}

	port, err := strconv.Atoi(u.Port())
	if err != nil {
		port = 6443
	}

	if k3s.Spec.ControlPlaneEndpoint == nil ||
		k3s.Spec.ControlPlaneEndpoint.Host != u.Hostname() ||
		k3s.Spec.ControlPlaneEndpoint.Port != port {
		k3s = k3s.DeepCopy()
		k3s.Spec.ControlPlaneEndpoint = &v1.Endpoint{
			Host: u.Hostname(),
			Port: port,
		}
		k3s, err = c.k3sClient.Update(k3s)
		if err != nil {
			return k3s, err
		}
	}

	return k3s, nil
}

func (c *handler) OnK3sChange(k3s *v1.K3s, status v1.K3sStatus) (_ []runtime.Object, ret v1.K3sStatus, err error) {
	version, err := c.getVersion(k3s.Spec.Channel)
	if err != nil {
		return nil, status, nil
	}

	caSecret, clientSecret, err := c.getSecrets(k3s)
	if err != nil {
		return nil, status, err
	}

	if status.Token == "" {
		status.Token, err = randomtoken.Generate()
		if err != nil {
			return nil, status, err
		}
	}

	status.CredentialSecretName = caSecret.Name
	status.ObservedGeneration = k3s.Generation

	return []runtime.Object{
		caSecret,
		clientSecret,
		&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: k3s.Namespace,
				Name:      k3s.Name,
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name:     "https",
						Protocol: corev1.ProtocolTCP,
						Port:     6443,
						TargetPort: intstr.IntOrString{
							IntVal: 6443,
						},
					},
				},
				Selector: map[string]string{
					"app": k3s.Name,
				},
				Type: corev1.ServiceTypeClusterIP,
			},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: k3s.Namespace,
				Name:      k3s.Name,
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": k3s.Name,
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": k3s.Name,
						},
					},
					Spec: corev1.PodSpec{
						Volumes: []corev1.Volume{
							{
								Name: "certs",
								VolumeSource: corev1.VolumeSource{
									Projected: &corev1.ProjectedVolumeSource{
										Sources: []corev1.VolumeProjection{
											{
												Secret: &corev1.SecretProjection{
													LocalObjectReference: corev1.LocalObjectReference{
														Name: caSecret.Name,
													},
													Items: []corev1.KeyToPath{
														{
															Path: "server-ca.crt",
															Key:  corev1.TLSCertKey,
														},
														{
															Path: "server-ca.key",
															Key:  corev1.TLSPrivateKeyKey,
														},
														{
															Path: "client-ca.crt",
															Key:  corev1.TLSCertKey,
														},
														{
															Path: "client-ca.key",
															Key:  corev1.TLSPrivateKeyKey,
														},
													},
												},
											},
										},
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:  "k3s",
								Image: "rancher/k3s:" + version,
								Args: []string{
									"server",
									"--cluster-cidr", "10.44.0.0/16",
									"--service-cidr", "10.45.0.0/16",
								},
								WorkingDir: "/var/lib/rancher/k3s",
								Env: []corev1.EnvVar{
									{
										Name:  "K3S_TOKEN",
										Value: status.Token,
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "certs",
										MountPath: "/var/lib/rancher/k3s/server/tls/server-ca.crt",
										SubPath:   "server-ca.crt",
									},
									{
										Name:      "certs",
										MountPath: "/var/lib/rancher/k3s/server/tls/server-ca.key",
										SubPath:   "server-ca.key",
									},
									{
										Name:      "certs",
										MountPath: "/var/lib/rancher/k3s/server/tls/client-ca.crt",
										SubPath:   "client-ca.crt",
									},
									{
										Name:      "certs",
										MountPath: "/var/lib/rancher/k3s/server/tls/client-ca.key",
										SubPath:   "client-ca.key",
									},
								},
								ReadinessProbe: &corev1.Probe{
									Handler: corev1.Handler{
										HTTPGet: &corev1.HTTPGetAction{
											Path: "/ping",
											Port: intstr.IntOrString{
												IntVal: 6443,
											},
											Scheme: corev1.URISchemeHTTPS,
										},
									},
									InitialDelaySeconds: 2,
								},
								SecurityContext: &corev1.SecurityContext{
									Privileged: &t,
								},
								Resources: corev1.ResourceRequirements{
									Requests: map[corev1.ResourceName]resource.Quantity{
										corev1.ResourceCPU: *resource.NewMilliQuantity(100, resource.BinarySI),
									},
								},
							},
						},
						Hostname: k3s.Name,
					},
				},
			},
		},
	}, status, nil
}

func (c *handler) getSecrets(k3s *v1.K3s) (*corev1.Secret, *corev1.Secret, error) {
	var (
		caName     = k3s.Name + "-kubeconfig"
		clientName = k3s.Name + "-kubeconfig-client"
	)

	ca, caKey, err := tlsstorage.LoadOrGenCA(c.secrets, k3s.Namespace, caName)
	if err != nil {
		return nil, nil, err
	}

	caPem, caKeyPem, err := factory.Marshal(ca, caKey)
	if err != nil {
		return nil, nil, err
	}

	clientCert, clientKey, err := tlsstorage.LoadOrGenClient(c.secrets, k3s.Namespace, clientName, "system:admin,o=system:masters", ca, caKey)
	if err != nil {
		return nil, nil, err
	}

	clientPem, clientKeyPem, err := factory.Marshal(clientCert, clientKey)
	if err != nil {
		return nil, nil, err
	}

	kubeConfig, err := clientcmd.Write(clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			"cluster": {
				Server:                   fmt.Sprintf("https://%s.%s:6443", k3s.Name, k3s.Namespace),
				CertificateAuthorityData: caPem,
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"user": {
				ClientCertificateData: clientPem,
				ClientKeyData:         clientKeyPem,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"default": {
				Cluster:  "cluster",
				AuthInfo: "user",
			},
		},
		CurrentContext: "default",
	})
	if err != nil {
		return nil, nil, err
	}

	return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: k3s.Namespace,
				Name:      caName,
			},
			Data: map[string][]byte{
				corev1.TLSCertKey:       caPem,
				corev1.TLSPrivateKeyKey: caKeyPem,
				"value":                 kubeConfig,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: k3s.Namespace,
				Name:      clientName,
			},
			Data: map[string][]byte{
				corev1.TLSCertKey:       clientPem,
				corev1.TLSPrivateKeyKey: clientKeyPem,
			},
		}, nil
}

func (c *handler) getVersion(channel string) (string, error) {
	if channel == "" {
		channel = "stable"
	}
	resp, err := client.Get("https://update.k3s.io/v1-release/channels/" + channel)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	return strings.ReplaceAll(path.Base(resp.Header.Get("Location")), "+", "-"), nil
}
