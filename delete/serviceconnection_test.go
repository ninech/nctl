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

func TestServiceConnection(t *testing.T) {
	ctx := context.Background()
	cmd := serviceConnectionCmd{
		resourceCmd: resourceCmd{
			Name:        "test",
			Force:       true,
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	sc := test.ServiceConnection("test", test.DefaultProject)

	apiClient, err := test.SetupClient()
	require.NoError(t, err)

	if err := apiClient.Create(ctx, sc); err != nil {
		t.Fatalf("serviceconnection create error, got: %s", err)
	}
	if err := apiClient.Get(ctx, api.ObjectName(sc), sc); err != nil {
		t.Fatalf("expected serviceconnection to exist, got: %s", err)
	}
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}
	err = apiClient.Get(ctx, api.ObjectName(sc), sc)
	if err == nil {
		t.Fatalf("expected serviceconnection to be deleted, but exists")
	}
	if !errors.IsNotFound(err) {
		t.Fatalf("expected serviceconnection to be deleted, got: %s", err.Error())
	}
}
