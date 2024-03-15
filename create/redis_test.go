package create

import (
	"context"
	"testing"
	"time"

	"github.com/ninech/nctl/api"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRedis(t *testing.T) {
	cmd := redisCmd{
		Name:        "test",
		Wait:        false,
		WaitTimeout: time.Second,
	}

	redis := cmd.newRedis("default")
	redis.Name = "test"

	scheme, err := api.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	apiClient := &api.Client{WithWatch: client, Project: "default"}
	ctx := context.Background()

	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}

	if err := apiClient.Get(ctx, api.ObjectName(redis), redis); err != nil {
		t.Fatalf("expected redis to exist, got: %s", err)
	}
}
