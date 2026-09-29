package kubeconfig

import (
	"bytes"
	"testing"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/config"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

func TestLoginCluster(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	cluster := &infrastructure.KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Status: infrastructure.KubernetesClusterStatus{
			AtProvider: infrastructure.KubernetesClusterObservation{
				ClusterObservation: infrastructure.ClusterObservation{
					APIEndpoint:   "https://new.example.org",
					OIDCClientID:  "some-client-id",
					OIDCIssuerURL: "https://auth.example.org",
				},
			},
		},
	}
	// the test kubeconfig already contains one entry which has to survive
	// the merge
	apiClient := test.SetupClient(t,
		test.WithKubeconfig(),
		test.WithObjects(cluster),
	)

	out := &bytes.Buffer{}
	// we run without the execPlugin, that would be something for an e2e test
	is.NoError(LoginCluster(t.Context(), apiClient, format.NewWriter(out), api.ObjectName(cluster), false))

	merged, err := clientcmd.LoadFromFile(apiClient.KubeconfigPath)
	is.NoError(err)

	contextName := config.ContextName(cluster)
	is.Len(merged.Clusters, 2)
	is.Len(merged.Contexts, 2)
	is.Len(merged.AuthInfos, 2)
	is.Equal(contextName, merged.CurrentContext)
	is.Equal(cluster.Status.AtProvider.APIEndpoint, merged.Clusters[contextName].Server)
	is.Equal([]string{
		api.AuthCmdName,
		api.OIDCCmdName,
		api.IssuerURLArg + cluster.Status.AtProvider.OIDCIssuerURL,
		api.ClientIDArg + cluster.Status.AtProvider.OIDCClientID,
		api.UsePKCEArg,
	}, merged.AuthInfos[contextName].Exec.Args)
	is.Contains(out.String(), "added "+contextName+" to kubeconfig")
	is.Contains(out.String(), "logged into cluster "+contextName)
}

func TestLoginClusterNotFound(t *testing.T) {
	t.Parallel()

	apiClient := test.SetupClient(t, test.WithKubeconfig())
	err := LoginCluster(t.Context(), apiClient, format.Writer{}, api.ObjectName(&infrastructure.KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "missing", Namespace: test.DefaultProject},
	}), false)
	require.Error(t, err)
}
