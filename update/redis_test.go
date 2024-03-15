package update

import (
	"context"
	"reflect"
	"testing"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRedis(t *testing.T) {
	max := storage.RedisMaxMemoryPolicy("noeviction")
	cmd := redisCmd{
		Name: "test",
		MaxMemoryPolicy: &max,
	}

	redis :=  test.Redis("test", "default", "nine-es34")

	scheme, err := api.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	apiClient := &api.Client{WithWatch: fake.NewClientBuilder().WithScheme(scheme).Build(), Project: "default"}
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
	if err := apiClient.Get(ctx, api.ObjectName(redis), redis); err != nil {
		t.Fatalf("expected redis to exist, got: %s", err)
	}
	if !reflect.DeepEqual(redis.Spec.ForProvider.MemorySize, cmd.MemorySize) {
		t.Fatalf("expected redis.Spec.ForProvider.MemorySize = %v, got: %v", cmd.MemorySize, redis.Spec.ForProvider.MemorySize)
	}
	if cmd.MaxMemoryPolicy != nil && !reflect.DeepEqual(redis.Spec.ForProvider.MaxMemoryPolicy, *cmd.MaxMemoryPolicy) {
		t.Fatalf("expected redis.Spec.ForProvider.MaxMemoryPolicy = %v, got: %v", cmd.MemorySize, redis.Spec.ForProvider.MemorySize)
	}
	if cmd.AllowedCIDRs != nil &&!reflect.DeepEqual(redis.Spec.ForProvider.AllowedCIDRs, *cmd.AllowedCIDRs) {
		t.Fatalf("expected redis.Spec.ForProvider.AllowedCIDRs = %v, got: %v", cmd.MemorySize, redis.Spec.ForProvider.MemorySize)
	}
}
