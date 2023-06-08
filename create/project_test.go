package create

import (
	"context"
	"os"
	"testing"
	"time"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/auth"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestProjects(t *testing.T) {
	ctx := context.Background()
	projectName, contextName, organization := "testproject", "test", "evilcorp"
	apiClient, err := test.SetupClient()
	if err != nil {
		t.Fatal(err)
	}
	kubeconfigPath, err := createTempKubeconfig(contextName, organization)
	require.NoError(t, err)
	defer os.Remove(kubeconfigPath)
	apiClient.KubeconfigPath = kubeconfigPath
	apiClient.KubeconfigContext = contextName

	cmd := projectCmd{
		Name:        projectName,
		Wait:        false,
		WaitTimeout: time.Second,
	}

	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}

	if err := apiClient.Get(
		ctx,
		api.NamespacedName(projectName, organization),
		&management.Project{},
	); err != nil {
		t.Fatalf("expected project %q to exist, got: %s", "testproject", err)
	}
}

func createTempKubeconfig(contextName, organization string) (string, error) {
	cfg := auth.NewConfig(organization)
	cfgObject, err := cfg.ToObject()
	if err != nil {
		return "", err
	}
	// create and open a temporary file
	f, err := os.CreateTemp("", "kubeconfig-")
	if err != nil {
		return "", err
	}
	defer f.Close()

	kubeconfig := clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			contextName: {
				Server: "not.so.important",
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			contextName: {
				Token: "not-valid",
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			contextName: {
				Cluster:   contextName,
				AuthInfo:  contextName,
				Namespace: "default",
				Extensions: map[string]runtime.Object{
					auth.NctlExtensionName: cfgObject,
				},
			},
		},
		CurrentContext: contextName,
	}

	content, err := clientcmd.Write(kubeconfig)
	if err != nil {
		return "", err
	}
	if _, err = f.Write(content); err != nil {
		return "", err
	}

	return f.Name(), nil
}
