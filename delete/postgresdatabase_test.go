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

func TestPostgresDatabase(t *testing.T) {
	ctx := context.Background()
	cmd := postgresDatabaseCmd{
		resourceCmd: resourceCmd{
			Name:        "test",
			Force:       true,
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	postgresDatabase := test.PostgresDatabase("test", test.DefaultProject, "nine-es34")

	apiClient, err := test.SetupClient()
	require.NoError(t, err)

	if err := apiClient.Create(ctx, postgresDatabase); err != nil {
		t.Fatalf("postgresdatabase create error, got: %s", err)
	}
	if err := apiClient.Get(ctx, api.ObjectName(postgresDatabase), postgresDatabase); err != nil {
		t.Fatalf("expected postgresdatabase to exist, got: %s", err)
	}
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}
	err = apiClient.Get(ctx, api.ObjectName(postgresDatabase), postgresDatabase)
	if err == nil {
		t.Fatalf("expected postgresdatabase to be deleted, but exists")
	}
	if !errors.IsNotFound(err) {
		t.Fatalf("expected postgresdatabase to be deleted, got: %s", err.Error())
	}
}
