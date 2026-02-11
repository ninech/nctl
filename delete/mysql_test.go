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

func TestMySQL(t *testing.T) {
	t.Parallel()
	out := &bytes.Buffer{}
	cmd := mySQLCmd{
		resourceCmd: resourceCmd{
			Writer:      format.NewWriter(out),
			Name:        "test",
			Force:       true,
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	mysql := test.MySQL("test", test.DefaultProject, "nine-es34")
	apiClient := test.SetupClient(t)

	ctx := t.Context()
	if err := apiClient.Create(ctx, mysql); err != nil {
		t.Fatalf("failed to create mysql: %v", err)
	}
	if err := apiClient.Get(ctx, api.ObjectName(mysql), mysql); err != nil {
		t.Fatalf("expected mysql to exist before deletion, got error: %v", err)
	}
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatalf("failed to run mysql delete command: %v", err)
	}
	err := apiClient.Get(ctx, api.ObjectName(mysql), mysql)
	if err == nil {
		t.Fatal("expected mysql to be deleted, but it still exists")
	}
	if !kerrors.IsNotFound(err) {
		t.Fatalf("expected mysql to be deleted (NotFound), but got error: %v", err)
	}

	if !strings.Contains(out.String(), "deletion started") {
		t.Errorf("expected output to contain 'deletion started', got %q", out.String())
	}
	if !strings.Contains(out.String(), cmd.Name) {
		t.Errorf("expected output to contain mysql name %q, got %q", cmd.Name, out.String())
	}
}
