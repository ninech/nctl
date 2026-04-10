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

func TestGrafana(t *testing.T) {
	t.Parallel()
	out := &bytes.Buffer{}
	cmd := grafanaCmd{
		resourceCmd: resourceCmd{
			Writer:      format.NewWriter(out),
			Name:        "test",
			Force:       true,
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	grafana := test.Grafana("test", test.DefaultProject)

	apiClient := test.SetupClient(t)

	ctx := t.Context()
	if err := apiClient.Create(ctx, grafana); err != nil {
		t.Fatalf("failed to create grafana: %v", err)
	}
	if err := apiClient.Get(ctx, api.ObjectName(grafana), grafana); err != nil {
		t.Fatalf("expected grafana to exist before deletion, got error: %v", err)
	}
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatalf("failed to run grafana delete command: %v", err)
	}
	err := apiClient.Get(ctx, api.ObjectName(grafana), grafana)
	if err == nil {
		t.Fatal("expected grafana to be deleted, but it still exists")
	}
	if !kerrors.IsNotFound(err) {
		t.Fatalf("expected grafana to be deleted (NotFound), but got error: %v", err)
	}

	if !strings.Contains(out.String(), "deletion started") {
		t.Errorf("expected output to contain 'deletion started', got %q", out.String())
	}
	if !strings.Contains(out.String(), cmd.Name) {
		t.Errorf("expected output to contain grafana name %q, got %q", cmd.Name, out.String())
	}
}
