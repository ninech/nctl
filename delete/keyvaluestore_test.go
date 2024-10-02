package delete

import (
	"context"
	"testing"
	"time"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
)

func TestKeyValueStore(t *testing.T) {
	ctx := context.Background()
	cmd := keyValueStoreCmd{
		resourceCmd: resourceCmd{
			Name:        "test",
			Force:       true,
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	keyValueStore := test.KeyValueStore("test", test.DefaultProject, "nine-es34")

	apiClient, err := test.SetupClient()
	require.NoError(t, err)

	if err := apiClient.Create(ctx, keyValueStore); err != nil {
		t.Fatalf("keyvaluestore create error, got: %s", err)
	}
	if err := apiClient.Get(ctx, api.ObjectName(keyValueStore), keyValueStore); err != nil {
		t.Fatalf("expected keyvaluestore to exist, got: %s", err)
	}
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}
	err = apiClient.Get(ctx, api.ObjectName(keyValueStore), keyValueStore)
	if err == nil {
		t.Fatalf("expected keyvaluestore to be deleted, but exists")
	}
	if !errors.IsNotFound(err) {
		t.Fatalf("expected keyvaluestore to be deleted, got: %s", err.Error())
	}
}
