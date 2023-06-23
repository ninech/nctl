package api

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/ninech/apis"
	"github.com/ninech/nctl/api/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Client struct {
	runtimeclient.WithWatch
	Config            *rest.Config
	KubeconfigPath    string
	Project           string
	Log               *log.Client
	Token             string
	KubeconfigContext string
}

type ClientOpt func(c *Client) error

// New returns a new Client by loading a kubeconfig with the supplied context
// and project. The kubeconfig is discovered like this:
// * KUBECONFIG environment variable pointing at a file
// * $HOME/.kube/config if exists
func New(ctx context.Context, apiClusterContext, project string, opts ...ClientOpt) (*Client, error) {
	client := &Client{
		Project:           project,
		KubeconfigContext: apiClusterContext,
	}
	if err := client.loadConfig(apiClusterContext); err != nil {
		return nil, err
	}

	token, err := GetTokenFromConfig(ctx, client.Config)
	if err != nil {
		return nil, err
	}
	client.Token = token

	scheme, err := NewScheme()
	if err != nil {
		return nil, err
	}

	mapper := apis.StaticRESTMapper(scheme)
	mapper.Add(corev1.SchemeGroupVersion.WithKind("Secret"), meta.RESTScopeNamespace)

	c, err := runtimeclient.NewWithWatch(client.Config, runtimeclient.Options{
		Scheme: scheme,
		Mapper: mapper,
	})
	if err != nil {
		return nil, err
	}
	client.WithWatch = c

	for _, opt := range opts {
		if err := opt(client); err != nil {
			return nil, err
		}
	}

	return client, nil
}

// LogClient sets up a log client connected to the provided address.
func LogClient(address string, insecure bool) ClientOpt {
	return func(c *Client) error {
		logClient, err := log.NewClient(address, c.Token, c.Project, insecure)
		if err != nil {
			return fmt.Errorf("unable to create log client: %w", err)
		}
		c.Log = logClient
		return nil
	}
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
func (c *Client) loadConfig(context string) error {
	loadingRules, err := LoadingRules()
	if err != nil {
		return err
	}

	cfg, project, err := loadConfigWithContext("", loadingRules, context)
	if err != nil {
		return err
	}
	if c.Project == "" {
		c.Project = project
	}
	c.Config = cfg
	c.KubeconfigPath = loadingRules.GetDefaultFilename()

	return nil
}

func (c *Client) Name(name string) types.NamespacedName {
	return types.NamespacedName{Name: name, Namespace: c.Project}
}

func (c *Client) GetConnectionSecret(ctx context.Context, mg resource.Managed) (*corev1.Secret, error) {
	if mg.GetWriteConnectionSecretToReference() == nil {
		return nil, fmt.Errorf("%T %s/%s has no connection secret ref set", mg, mg.GetName(), mg.GetNamespace())
	}

	nsName := types.NamespacedName{
		Name:      mg.GetWriteConnectionSecretToReference().Name,
		Namespace: mg.GetWriteConnectionSecretToReference().Namespace,
	}
	secret := &corev1.Secret{}
	if err := c.Get(ctx, nsName, secret); err != nil {
		return nil, fmt.Errorf("unable to get referenced secret %v: %w", nsName, err)
	}

	return secret, nil
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

func ObjectName(obj runtimeclient.Object) types.NamespacedName {
	return types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}
}

func NamespacedName(name, project string) types.NamespacedName {
	return types.NamespacedName{Name: name, Namespace: project}
}
