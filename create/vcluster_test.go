package create

import (
	"testing"
	"time"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/testutil"
)

func TestVCluster(t *testing.T) {
	t.Parallel()

	cmd := vclusterCmd{
		ResourceCmd: ResourceCmd{
			Name:        "falcon",
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	cluster := cmd.newCluster(testutil.DefaultProject)
	apiClient := testutil.SetupClient(t)

	if err := cmd.Run(t.Context(), apiClient); err != nil {
		t.Fatal(err)
	}

	if err := apiClient.Get(t.Context(), api.ObjectName(cluster), cluster); err != nil {
		t.Fatalf("expected vcluster to exist, got: %s", err)
	}
}
