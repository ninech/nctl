package delete

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

func TestOpenSearch(t *testing.T) {
	t.Parallel()
	out := &bytes.Buffer{}
	cmd := openSearchCmd{
		resourceCmd: resourceCmd{
			Writer:      format.NewWriter(out),
			Name:        "test",
			Force:       true,
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	opensearch := test.OpenSearch("test", test.DefaultProject, "nine-es34")

	apiClient := test.SetupClient(t)

	ctx := t.Context()
	if err := apiClient.Create(ctx, opensearch); err != nil {
		t.Fatalf("failed to create opensearch: %v", err)
	}
	if err := apiClient.Get(ctx, api.ObjectName(opensearch), opensearch); err != nil {
		t.Fatalf("expected opensearch to exist before deletion, got error: %v", err)
	}
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatalf("failed to run opensearch delete command: %v", err)
	}
	err := apiClient.Get(ctx, api.ObjectName(opensearch), opensearch)
	if err == nil {
		t.Fatal("expected opensearch to be deleted, but it still exists")
	}
	if !kerrors.IsNotFound(err) {
		t.Fatalf("expected opensearch to be deleted (NotFound), but got error: %v", err)
	}

	if !strings.Contains(out.String(), "deletion started") {
		t.Errorf("expected output to contain 'deletion started', got %q", out.String())
	}
	if !strings.Contains(out.String(), cmd.Name) {
		t.Errorf("expected output to contain opensearch name %q, got %q", cmd.Name, out.String())
	}
}
