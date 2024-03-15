package update

import (
	"context"
	"reflect"
	"testing"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRedis(t *testing.T) {
	tests := []struct {
		name    string
		create  storage.RedisParameters
		update  redisCmd
		want    storage.RedisParameters
		wantErr bool
	}{
		{"simple", storage.RedisParameters{}, redisCmd{}, storage.RedisParameters{}, false},
		{
			"memorySize",
			storage.RedisParameters{},
			redisCmd{MemorySize: ptr.To("1G")},
			storage.RedisParameters{MemorySize: memorySize("1G")},
			false,
		},
		{
			"memorySize",
			storage.RedisParameters{MemorySize: memorySize("2G")},
			redisCmd{MemorySize: ptr.To("1G")},
			storage.RedisParameters{MemorySize: memorySize("1G")},
			false,
		},
		{
			"invalid",
			storage.RedisParameters{MemorySize: memorySize("2G")},
			redisCmd{MemorySize: ptr.To("invalid")},
			storage.RedisParameters{MemorySize: memorySize("2G")},
			true,
		},
		{
			"maxMemoryPolicy",
			storage.RedisParameters{},
			redisCmd{MaxMemoryPolicy: ptr.To(storage.RedisMaxMemoryPolicy("noeviction"))},
			storage.RedisParameters{MaxMemoryPolicy: storage.RedisMaxMemoryPolicy("noeviction")},
			false,
		},
		{
			"maxMemoryPolicy",
			storage.RedisParameters{MaxMemoryPolicy: storage.RedisMaxMemoryPolicy("allkeys-lfu")},
			redisCmd{MaxMemoryPolicy: ptr.To(storage.RedisMaxMemoryPolicy("noeviction"))},
			storage.RedisParameters{MaxMemoryPolicy: storage.RedisMaxMemoryPolicy("noeviction")},
			false,
		},
		{
			"allowedCIDRs",
			storage.RedisParameters{},
			redisCmd{AllowedCIDRs: &[]storage.IPv4CIDR{storage.IPv4CIDR("0.0.0.0/0")}},
			storage.RedisParameters{AllowedCIDRs: []storage.IPv4CIDR{storage.IPv4CIDR("0.0.0.0/0")}},
			false,
		},
		{
			"allowedCIDRs",
			storage.RedisParameters{AllowedCIDRs: []storage.IPv4CIDR{"192.168.0.1/24"}},
			redisCmd{AllowedCIDRs: &[]storage.IPv4CIDR{storage.IPv4CIDR("0.0.0.0/0")}},
			storage.RedisParameters{AllowedCIDRs: []storage.IPv4CIDR{storage.IPv4CIDR("0.0.0.0/0")}},
			false,
		},
		{
			"allowedCIDRs",
			storage.RedisParameters{AllowedCIDRs: []storage.IPv4CIDR{"0.0.0.0/0"}},
			redisCmd{MemorySize: ptr.To("1G")},
			storage.RedisParameters{MemorySize: memorySize("1G"), AllowedCIDRs: []storage.IPv4CIDR{storage.IPv4CIDR("0.0.0.0/0")}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.update.Name = "test-" + t.Name()

			scheme, err := api.NewScheme()
			if err != nil {
				t.Fatal(err)
			}
			apiClient := &api.Client{WithWatch: fake.NewClientBuilder().WithScheme(scheme).Build(), Project: "default"}
			ctx := context.Background()

			created := test.Redis(tt.update.Name, apiClient.Project, "nine-es34")
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(ctx, created); err != nil {
				t.Fatalf("redis create error, got: %s", err)
			}
			if err := apiClient.Get(ctx, api.ObjectName(created), created); err != nil {
				t.Fatalf("expected redis to exist, got: %s", err)
			}

			updated := &storage.Redis{ObjectMeta: metav1.ObjectMeta{Name: created.Name, Namespace: created.Namespace}}
			if err := tt.update.Run(ctx, apiClient); (err != nil) != tt.wantErr {
				t.Errorf("redisCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := apiClient.Get(ctx, api.ObjectName(updated), updated); err != nil {
				t.Fatalf("expected redis to exist, got: %s", err)
			}

			if !reflect.DeepEqual(updated.Spec.ForProvider, tt.want) {
				t.Fatalf("expected redis.Spec.ForProvider = %v, got: %v", updated.Spec.ForProvider, tt.want)
			}
		})
	}
}

func memorySize(s string) *storage.RedisMemorySize {
	return &storage.RedisMemorySize{Quantity: resource.MustParse(s)}
}
