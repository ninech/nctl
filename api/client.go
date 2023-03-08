package api

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/ninech/apis"
	iam "github.com/ninech/apis/iam/v1alpha1"
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Client struct {
	runtimeclient.WithWatch
	Config         *rest.Config
	KubeconfigPath string
	Namespace      string
}

// New returns a new Client by loading a kubeconfig with the supplied context
// and namespace. The kubeconfig is discovered like this:
// * KUBECONFIG environment variable pointing at a file
// * $HOME/.kube/config if exists
func New(apiClusterContext, namespace string) (*Client, error) {
	client := &Client{
		Namespace: namespace,
	}
	if err := client.loadConfig(apiClusterContext); err != nil {
		return nil, err
	}

	scheme, err := NewScheme()
	if err != nil {
		return nil, err
	}

	// we statically define a RESTMapper in order to avoid slow discovery
	mapper := meta.NewDefaultRESTMapper(scheme.PrioritizedVersionsAllGroups())
	mapper.Add(infrastructure.KubernetesClusterGroupVersionKind, meta.RESTScopeNamespace)
	mapper.Add(iam.APIServiceAccountGroupVersionKind, meta.RESTScopeNamespace)
	mapper.Add(corev1.SchemeGroupVersion.WithKind("Secret"), meta.RESTScopeNamespace)

	c, err := runtimeclient.NewWithWatch(client.Config, runtimeclient.Options{Scheme: scheme, Mapper: mapper})
	if err != nil {
		return nil, err
	}

	client.WithWatch = c
	return client, nil
}

// NewScheme returns a *runtime.Scheme with all the relevant types registered.
func NewScheme() (*runtime.Scheme, error) {
	scheme := runtime.NewScheme()
	if err := apis.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	return scheme, nil
}

// adapted from https://github.com/kubernetes-sigs/controller-runtime/blob/4c9c9564e4652bbdec14a602d6196d8622500b51/pkg/client/config/config.go#L116
func (n *Client) loadConfig(context string) error {
	loadingRules, err := LoadingRules()
	if err != nil {
		return err
	}

	cfg, namespace, err := loadConfigWithContext("", loadingRules, context)
	if err != nil {
		return err
	}
	if n.Namespace == "" {
		n.Namespace = namespace
	}
	n.Config = cfg
	n.KubeconfigPath = loadingRules.GetDefaultFilename()

	return nil
}

func LoadingRules() (*clientcmd.ClientConfigLoadingRules, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if _, ok := os.LookupEnv("HOME"); !ok {
		u, err := user.Current()
		if err != nil {
			return nil, fmt.Errorf("could not get current user: %w", err)
		}
		loadingRules.Precedence = append(
			loadingRules.Precedence,
			filepath.Join(u.HomeDir, clientcmd.RecommendedHomeDir, clientcmd.RecommendedFileName),
		)
	}

	return loadingRules, nil
}

func loadConfigWithContext(apiServerURL string, loader clientcmd.ClientConfigLoader, context string) (*rest.Config, string, error) {
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loader, &clientcmd.ConfigOverrides{
			ClusterInfo: clientcmdapi.Cluster{
				Server: apiServerURL,
			},
			CurrentContext: context,
		},
	)

	ns, _, err := clientConfig.Namespace()
	if err != nil {
		return nil, "", err
	}

	cfg, err := clientConfig.ClientConfig()
	return cfg, ns, err
}
