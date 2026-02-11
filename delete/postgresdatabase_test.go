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

func TestPostgresDatabase(t *testing.T) {
	t.Parallel()
	out := &bytes.Buffer{}
	cmd := postgresDatabaseCmd{
		resourceCmd: resourceCmd{
			Writer:      format.NewWriter(out),
			Name:        "test",
			Force:       true,
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	postgresDatabase := test.PostgresDatabase("test", test.DefaultProject, "nine-es34")

	apiClient := test.SetupClient(t)

	ctx := t.Context()
	if err := apiClient.Create(ctx, postgresDatabase); err != nil {
		t.Fatalf("failed to create postgresdatabase: %v", err)
	}
	if err := apiClient.Get(ctx, api.ObjectName(postgresDatabase), postgresDatabase); err != nil {
		t.Fatalf("expected postgresdatabase to exist before deletion, got error: %v", err)
	}
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatalf("failed to run postgresdatabase delete command: %v", err)
	}
	err := apiClient.Get(ctx, api.ObjectName(postgresDatabase), postgresDatabase)
	if err == nil {
		t.Fatal("expected postgresdatabase to be deleted, but it still exists")
	}
	if !kerrors.IsNotFound(err) {
		t.Fatalf("expected postgresdatabase to be deleted (NotFound), but got error: %v", err)
	}

	if !strings.Contains(out.String(), "deletion started") {
		t.Errorf("expected output to contain 'deletion started', got %q", out.String())
	}
	if !strings.Contains(out.String(), cmd.Name) {
		t.Errorf("expected output to contain postgresdatabase name %q, got %q", cmd.Name, out.String())
	}
}
