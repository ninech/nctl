package create

import (
	"context"
	"testing"
	"time"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
)

func TestAPIServiceAccount(t *testing.T) {
	ctx := context.Background()
	cmd := apiServiceAccountCmd{
		resourceCmd: resourceCmd{
			Name:        "test",
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	asa := cmd.newAPIServiceAccount("default")
	asa.Name = "test"

	apiClient, err := test.SetupClient()
	require.NoError(t, err)

	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}

	if err := apiClient.Get(ctx, api.ObjectName(asa), asa); err != nil {
		t.Fatalf("expected asa to exist, got: %s", err)
	}
}
