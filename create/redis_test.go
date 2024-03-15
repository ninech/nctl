package create

import (
	"context"
	"reflect"
	"testing"
	"time"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		{"maxMemoryPolicy", redisCmd{MaxMemoryPolicy: storage.RedisMaxMemoryPolicy("noeviction")}, false},
		{"allowedCIDRs", redisCmd{AllowedCIDRs: []storage.IPv4CIDR{storage.IPv4CIDR("0.0.0.0/0")}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cmd.Name = "test-" + t.Name()
			tt.cmd.Wait = false
			tt.cmd.WaitTimeout = time.Second

			scheme, err := api.NewScheme()
			if err != nil {
				t.Fatal(err)
			}
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			apiClient := &api.Client{WithWatch: client, Project: "default"}
			ctx := context.Background()

			if err := tt.cmd.Run(ctx, apiClient); (err != nil) != tt.wantErr {
				t.Errorf("redisCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			redis := &storage.Redis{
				ObjectMeta: metav1.ObjectMeta{
					Name:      tt.cmd.Name,
					Namespace: apiClient.Project,
				},
			}
			if err := apiClient.Get(ctx, api.ObjectName(redis), redis); (err != nil) != tt.wantErr {
				t.Fatalf("expected redis to exist, got: %s", err)
			}
			if !reflect.DeepEqual(redis.Spec.ForProvider.MemorySize, tt.cmd.MemorySize) {
				t.Fatalf("expected redis.Spec.ForProvider.MemorySize = %v, got: %v", tt.cmd.MemorySize, redis.Spec.ForProvider.MemorySize)
			}
			if !reflect.DeepEqual(redis.Spec.ForProvider.MaxMemoryPolicy, tt.cmd.MaxMemoryPolicy) {
				t.Fatalf("expected redis.Spec.ForProvider.MaxMemoryPolicy = %v, got: %v", tt.cmd.MemorySize, redis.Spec.ForProvider.MemorySize)
			}
			if !reflect.DeepEqual(redis.Spec.ForProvider.AllowedCIDRs, tt.cmd.AllowedCIDRs) {
				t.Fatalf("expected redis.Spec.ForProvider.AllowedCIDRs = %v, got: %v", tt.cmd.MemorySize, redis.Spec.ForProvider.MemorySize)
			}
		})
	}
}
