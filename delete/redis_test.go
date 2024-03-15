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

func TestRedis(t *testing.T) {
	cmd := redisCmd{
		Name:        "test",
		Force:       true,
		Wait:        false,
		WaitTimeout: time.Second,
	}

	redis := test.Redis("test", "default", "nine-es34")

	scheme, err := api.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	apiClient := &api.Client{WithWatch: client, Project: "default"}
	ctx := context.Background()

	if err := apiClient.Create(ctx, redis); err != nil {
		t.Fatalf("redis create error, got: %s", err)
	}
	if err := apiClient.Get(ctx, api.ObjectName(redis), redis); err != nil {
		t.Fatalf("expected redis to exist, got: %s", err)
	}
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}
	err = apiClient.Get(ctx, api.ObjectName(redis), redis)
	if err == nil {
		t.Fatalf("expected redis to be deleted, but exists")
	}
	if !errors.IsNotFound(err) {
		t.Fatalf("expected redis to be deleted, got: %s", err.Error())
	}
}
