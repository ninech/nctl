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

func TestMySQL(t *testing.T) {
	ctx := context.Background()
	cmd := mySQLCmd{
		resourceCmd: resourceCmd{
			Name:        "test",
			Force:       true,
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	mysql := test.MySQL("test", test.DefaultProject, "nine-es34")
	apiClient, err := test.SetupClient()
	require.NoError(t, err)

	if err := apiClient.Create(ctx, mysql); err != nil {
		t.Fatalf("mysql create error, got: %s", err)
	}
	if err := apiClient.Get(ctx, api.ObjectName(mysql), mysql); err != nil {
		t.Fatalf("expected mysql to exist, got: %s", err)
	}
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}
	err = apiClient.Get(ctx, api.ObjectName(mysql), mysql)
	if err == nil {
		t.Fatalf("expected mysql to be deleted, but exists")
	}
	if !errors.IsNotFound(err) {
		t.Fatalf("expected mysql to be deleted, got: %s", err.Error())
	}
}
