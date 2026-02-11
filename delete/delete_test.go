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

func TestDeleter(t *testing.T) {
	t.Parallel()
	asa := &iam.APIServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: test.DefaultProject,
		},
		Spec: iam.APIServiceAccountSpec{},
	}
	apiClient := test.SetupClient(t,
		test.WithObjects(asa),
	)
	out := &bytes.Buffer{}
	cmd := &apiServiceAccountCmd{resourceCmd{Writer: format.NewWriter(out)}}

	d := cmd.newDeleter(asa, iam.APIServiceAccountKind)

	ctx := t.Context()
	if err := d.deleteResource(ctx, apiClient, 0, false, true); err != nil {
		t.Fatalf("failed to delete resource: %v", err)
	}

	if !kerrors.IsNotFound(apiClient.Get(ctx, api.ObjectName(asa), asa)) {
		t.Fatal("expected resource to be deleted, but it still exists")
	}

	if !strings.Contains(out.String(), "deletion started") {
		t.Errorf("expected output to contain 'deletion started', got %q", out.String())
	}
}
