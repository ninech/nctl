package create

import (
	"testing"
	"time"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
)

func TestVCluster(t *testing.T) {
	t.Parallel()

	cmd := vclusterCmd{
		resourceCmd: resourceCmd{
			Name:        "falcon",
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	cluster := cmd.newCluster(test.DefaultProject)
	apiClient := test.SetupClient(t)

	if err := cmd.Run(t.Context(), apiClient); err != nil {
		t.Fatal(err)
	}

	if err := apiClient.Get(t.Context(), api.ObjectName(cluster), cluster); err != nil {
		t.Fatalf("expected vcluster to exist, got: %s", err)
	}
}
