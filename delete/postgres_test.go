package delete

import (
	"context"
	"testing"
	"time"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPostgres(t *testing.T) {
	cmd := postgresCmd{
		Name:        "test",
		Force:       true,
		Wait:        false,
		WaitTimeout: time.Second,
	}

	postgres := test.Postgres("test", "default", "nine-es34")

	scheme, err := api.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	apiClient := &api.Client{WithWatch: client, Project: "default"}
	ctx := context.Background()

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
