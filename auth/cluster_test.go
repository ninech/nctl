package auth

import (
	"io"
	"log"
	"os"
	"testing"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api/config"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"
)

const existingKubeconfig = `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://existing.example.org
  name: existing
users:
- name: existing
current-context: existing
contexts:
- context:
  name: existing
`

func TestClusterCmd(t *testing.T) {
	t.Parallel()

	// write our "existing" kubeconfig to a temp kubeconfig
	kubeconfig, err := os.CreateTemp("", "*-kubeconfig.yaml")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(kubeconfig.Name())

	if err := os.WriteFile(kubeconfig.Name(), []byte(existingKubeconfig), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	cluster := newCluster()
	is := require.New(t)
	apiClient, err := test.SetupClient(
		test.WithObjects(cluster),
	)
	is.NoError(err)
	apiClient.KubeconfigPath = kubeconfig.Name()

	// we run without the execPlugin, that would be something for an e2e test
	cmd := &ClusterCmd{Name: config.ContextName(cluster), ExecPlugin: false}
	if err := cmd.Run(t.Context(), apiClient); err != nil {
		t.Fatal(err)
	}

	// read out the kubeconfig again to test the contents
	b, err := io.ReadAll(kubeconfig)
	if err != nil {
		t.Fatal(err)
	}

	merged, err := clientcmd.Load(b)
	if err != nil {
		t.Fatal(err)
	}

	checkConfig(t, merged, 2, config.ContextName(cluster))
}

func newCluster() *infrastructure.KubernetesCluster {
	return &infrastructure.KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: infrastructure.KubernetesClusterSpec{},
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
}
