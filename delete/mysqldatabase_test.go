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

func TestMySQLDatabase(t *testing.T) {
	ctx := context.Background()
	cmd := mysqlDatabaseCmd{
		resourceCmd: resourceCmd{
			Name:        "test",
			Force:       true,
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	mysqlDatabase := test.MySQLDatabase("test", test.DefaultProject, "nine-es34")

	apiClient, err := test.SetupClient()
	require.NoError(t, err)

	if err := apiClient.Create(ctx, mysqlDatabase); err != nil {
		t.Fatalf("mysqldatabase create error, got: %s", err)
	}
	if err := apiClient.Get(ctx, api.ObjectName(mysqlDatabase), mysqlDatabase); err != nil {
		t.Fatalf("expected mysqldatabase to exist, got: %s", err)
	}
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}
	err = apiClient.Get(ctx, api.ObjectName(mysqlDatabase), mysqlDatabase)
	if err == nil {
		t.Fatalf("expected mysqldatabase to be deleted, but exists")
	}
	if !errors.IsNotFound(err) {
		t.Fatalf("expected mysqldatabase to be deleted, got: %s", err.Error())
	}
}
