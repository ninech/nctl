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

func TestStaticEgress(t *testing.T) {
	t.Parallel()
	out := &bytes.Buffer{}
	cmd := staticEgressCmd{
		resourceCmd: resourceCmd{
			Writer:      format.NewWriter(out),
			Name:        "test",
			Force:       true,
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	staticEgress := test.StaticEgress("test", test.DefaultProject, "my-app")

	apiClient := test.SetupClient(t)

	ctx := t.Context()
	if err := apiClient.Create(ctx, staticEgress); err != nil {
		t.Fatalf("failed to create static egress: %v", err)
	}
	if err := apiClient.Get(ctx, api.ObjectName(staticEgress), staticEgress); err != nil {
		t.Fatalf("expected static egress to exist before deletion, got error: %v", err)
	}
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatalf("failed to run static egress delete command: %v", err)
	}
	err := apiClient.Get(ctx, api.ObjectName(staticEgress), staticEgress)
	if err == nil {
		t.Fatal("expected static egress to be deleted, but it still exists")
	}
	if !kerrors.IsNotFound(err) {
		t.Fatalf("expected static egress to be deleted (NotFound), but got error: %v", err)
	}

	if !strings.Contains(out.String(), "deletion started") {
		t.Errorf("expected output to contain 'deletion started', got %q", out.String())
	}
	if !strings.Contains(out.String(), cmd.Name) {
		t.Errorf("expected output to contain static egress name %q, got %q", cmd.Name, out.String())
	}
}
