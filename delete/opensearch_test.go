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

func TestOpenSearch(t *testing.T) {
	ctx := context.Background()
	cmd := openSearchCmd{
		resourceCmd: resourceCmd{
			Name:        "test",
			Force:       true,
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	opensearch := test.OpenSearch("test", test.DefaultProject, "nine-es34")

	apiClient, err := test.SetupClient()
	require.NoError(t, err)

	if err := apiClient.Create(ctx, opensearch); err != nil {
		t.Fatalf("opensearch create error, got: %s", err)
	}
	if err := apiClient.Get(ctx, api.ObjectName(opensearch), opensearch); err != nil {
		t.Fatalf("expected opensearch to exist, got: %s", err)
	}
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}
	err = apiClient.Get(ctx, api.ObjectName(opensearch), opensearch)
	if err == nil {
		t.Fatalf("expected opensearch to be deleted, but exists")
	}
	if !errors.IsNotFound(err) {
		t.Fatalf("expected opensearch to be deleted, got: %s", err.Error())
	}
}
