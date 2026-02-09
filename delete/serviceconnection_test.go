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

func TestServiceConnection(t *testing.T) {
	t.Parallel()
	out := &bytes.Buffer{}
	cmd := serviceConnectionCmd{
		resourceCmd: resourceCmd{
			Writer:      format.NewWriter(out),
			Name:        "test",
			Force:       true,
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	sc := test.ServiceConnection("test", test.DefaultProject)

	apiClient, err := test.SetupClient()
	if err != nil {
		t.Fatalf("failed to setup api client: %v", err)
	}

	ctx := t.Context()
	if err := apiClient.Create(ctx, sc); err != nil {
		t.Fatalf("failed to create serviceconnection: %v", err)
	}
	if err := apiClient.Get(ctx, api.ObjectName(sc), sc); err != nil {
		t.Fatalf("expected serviceconnection to exist before deletion, got error: %v", err)
	}
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatalf("failed to run serviceconnection delete command: %v", err)
	}
	err = apiClient.Get(ctx, api.ObjectName(sc), sc)
	if err == nil {
		t.Fatal("expected serviceconnection to be deleted, but it still exists")
	}
	if !kerrors.IsNotFound(err) {
		t.Fatalf("expected serviceconnection to be deleted (NotFound), but got error: %v", err)
	}

	if !strings.Contains(out.String(), "deletion started") {
		t.Errorf("expected output to contain 'deletion started', got %q", out.String())
	}
	if !strings.Contains(out.String(), cmd.Name) {
		t.Errorf("expected output to contain serviceconnection name %q, got %q", cmd.Name, out.String())
	}
}
