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

func TestPostgres(t *testing.T) {
	ctx := context.Background()
	cmd := postgresCmd{
		resourceCmd: resourceCmd{
			Name:        "test",
			Force:       true,
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	postgres := test.Postgres("test", test.DefaultProject, "nine-es34")

	apiClient, err := test.SetupClient()
	require.NoError(t, err)

	if err := apiClient.Create(ctx, postgres); err != nil {
		t.Fatalf("postgres create error, got: %s", err)
	}
	if err := apiClient.Get(ctx, api.ObjectName(postgres), postgres); err != nil {
		t.Fatalf("expected postgres to exist, got: %s", err)
	}
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}
	err = apiClient.Get(ctx, api.ObjectName(postgres), postgres)
	if err == nil {
		t.Fatalf("expected postgres to be deleted, but exists")
	}
	if !errors.IsNotFound(err) {
		t.Fatalf("expected postgres to be deleted, got: %s", err.Error())
	}
}
