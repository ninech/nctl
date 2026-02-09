package delete

import (
	"bytes"
	"strings"
	"testing"

	iam "github.com/ninech/apis/iam/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAPIServiceAccount(t *testing.T) {
	t.Parallel()
	out := &bytes.Buffer{}
	cmd := apiServiceAccountCmd{
		resourceCmd: resourceCmd{
			Writer: format.NewWriter(out),
			Name:   "test",
			Force:  true,
			Wait:   false,
		},
	}

	asa := &iam.APIServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: test.DefaultProject,
		},
	}
	apiClient, err := test.SetupClient(test.WithObjects(asa))
	if err != nil {
		t.Fatalf("failed to setup api client: %v", err)
	}

	ctx := t.Context()
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatalf("failed to run apiserviceaccount delete command: %v", err)
	}

	if !kerrors.IsNotFound(apiClient.Get(ctx, api.ObjectName(asa), asa)) {
		t.Fatal("expected apiserviceaccount to be deleted, but it still exists")
	}

	if !strings.Contains(out.String(), "deletion started") {
		t.Errorf("expected output to contain 'deletion started', got %q", out.String())
	}
	if !strings.Contains(out.String(), cmd.Name) {
		t.Errorf("expected output to contain apiserviceaccount name %q, got %q", cmd.Name, out.String())
	}
}
