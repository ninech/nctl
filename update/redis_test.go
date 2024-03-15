package update

import (
	"context"
	"reflect"
	"testing"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)


func Test_redisCmd_Run(t *testing.T) {
	tests := []struct {
		name    string
		cmd     redisCmd
		wantErr bool
	}{
		{"simple", redisCmd{}, false},
		{"memorySize", redisCmd{MemorySize: &storage.RedisMemorySize{Quantity: resource.MustParse("1G")}}, false},
		{"maxMemoryPolicy", redisCmd{MaxMemoryPolicy: ptr.To(storage.RedisMaxMemoryPolicy("noeviction"))}, false},
		{"allowedCIDRs", redisCmd{AllowedCIDRs: &[]storage.IPv4CIDR{storage.IPv4CIDR("0.0.0.0/0")}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cmd.Name = "test-" + t.Name()

			scheme, err := api.NewScheme()
			if err != nil {
				t.Fatal(err)
			}
			apiClient := &api.Client{WithWatch: fake.NewClientBuilder().WithScheme(scheme).Build(), Project: "default"}
			ctx := context.Background()

			redis :=  test.Redis(tt.cmd.Name, apiClient.Project, "nine-es34")
			if err := apiClient.Create(ctx, redis); err != nil {
				t.Fatalf("redis create error, got: %s", err)
			}
			if err := apiClient.Get(ctx, api.ObjectName(redis), redis); err != nil {
				t.Fatalf("expected redis to exist, got: %s", err)
			}

			if err := tt.cmd.Run(ctx, apiClient); (err != nil) != tt.wantErr {
				t.Errorf("redisCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err := apiClient.Get(ctx, api.ObjectName(redis), redis); (err != nil) != tt.wantErr {
				t.Fatalf("expected redis to exist, got: %s", err)
			}
			if !reflect.DeepEqual(redis.Spec.ForProvider.MemorySize, tt.cmd.MemorySize) {
				t.Fatalf("expected redis.Spec.ForProvider.MemorySize = %v, got: %v", tt.cmd.MemorySize, redis.Spec.ForProvider.MemorySize)
			}
			if tt.cmd.MaxMemoryPolicy != nil && !reflect.DeepEqual(redis.Spec.ForProvider.MaxMemoryPolicy, *tt.cmd.MaxMemoryPolicy) {
				t.Fatalf("expected redis.Spec.ForProvider.MaxMemoryPolicy = %v, got: %v", *tt.cmd.MaxMemoryPolicy, redis.Spec.ForProvider.MaxMemoryPolicy)
			}
			if tt.cmd.AllowedCIDRs != nil && !reflect.DeepEqual(redis.Spec.ForProvider.AllowedCIDRs, *tt.cmd.AllowedCIDRs) {
				t.Fatalf("expected redis.Spec.ForProvider.AllowedCIDRs = %v, got: %v", *tt.cmd.AllowedCIDRs, redis.Spec.ForProvider.AllowedCIDRs)
			}
		})
	}
}
