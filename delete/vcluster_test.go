package delete

import (
	"bytes"
	"os"
	"strings"
	"testing"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestVCluster(t *testing.T) {
	t.Parallel()
	out := &bytes.Buffer{}
	cmd := vclusterCmd{
		resourceCmd: resourceCmd{
			Writer: format.NewWriter(out),
			Name:   "test",
			Force:  true,
			Wait:   false,
		},
	}

	cluster := &infrastructure.KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: test.DefaultProject,
		},
		Spec: infrastructure.KubernetesClusterSpec{
			ForProvider: infrastructure.KubernetesClusterParameters{
				VCluster: &infrastructure.VClusterSettings{},
			},
		},
	}

	apiClient := test.SetupClient(t, test.WithObjects(cluster))

	kubeconfig, err := test.CreateTestKubeconfig(apiClient, "")
	if err != nil {
		t.Fatalf("failed to create test kubeconfig: %v", err)
	}
	defer os.Remove(kubeconfig)
	apiClient.KubeconfigPath = kubeconfig

	ctx := t.Context()
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatalf("failed to run vcluster delete command: %v", err)
	}

	if !kerrors.IsNotFound(apiClient.Get(ctx, api.ObjectName(cluster), cluster)) {
		t.Fatal("expected vcluster to be deleted, but it still exists")
	}

	if !strings.Contains(out.String(), "deletion started") {
		t.Errorf("expected output to contain 'deletion started', got %q", out.String())
	}
	if !strings.Contains(out.String(), cmd.Name) {
		t.Errorf("expected output to contain vcluster name %q, got %q", cmd.Name, out.String())
	}
}
