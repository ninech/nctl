package create

import (
	"context"
	"testing"
	"time"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
)

func TestVCluster(t *testing.T) {
	ctx := context.Background()
	cmd := vclusterCmd{
		resourceCmd: resourceCmd{
			Name:        "falcon",
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	cluster := cmd.newCluster(test.DefaultProject)
	apiClient, err := test.SetupClient()
	require.NoError(t, err)

	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}

	if err := apiClient.Get(ctx, api.ObjectName(cluster), cluster); err != nil {
		t.Fatalf("expected vcluster to exist, got: %s", err)
	}
}
