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

func TestKeyValueStore(t *testing.T) {
	t.Parallel()
	out := &bytes.Buffer{}
	cmd := keyValueStoreCmd{
		resourceCmd: resourceCmd{
			Writer:      format.NewWriter(out),
			Name:        "test",
			Force:       true,
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	keyValueStore := test.KeyValueStore("test", test.DefaultProject, "nine-es34")

	apiClient := test.SetupClient(t)

	ctx := t.Context()
	if err := apiClient.Create(ctx, keyValueStore); err != nil {
		t.Fatalf("failed to create keyvaluestore: %v", err)
	}
	if err := apiClient.Get(ctx, api.ObjectName(keyValueStore), keyValueStore); err != nil {
		t.Fatalf("expected keyvaluestore to exist before deletion, got error: %v", err)
	}
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatalf("failed to run keyvaluestore delete command: %v", err)
	}
	err := apiClient.Get(ctx, api.ObjectName(keyValueStore), keyValueStore)
	if err == nil {
		t.Fatal("expected keyvaluestore to be deleted, but it still exists")
	}
	if !kerrors.IsNotFound(err) {
		t.Fatalf("expected keyvaluestore to be deleted (NotFound), but got error: %v", err)
	}

	if !strings.Contains(out.String(), "deletion started") {
		t.Errorf("expected output to contain 'deletion started', got %q", out.String())
	}
	if !strings.Contains(out.String(), cmd.Name) {
		t.Errorf("expected output to contain keyvaluestore name %q, got %q", cmd.Name, out.String())
	}
}
