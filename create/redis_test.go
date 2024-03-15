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

func TestRedis(t *testing.T) {
	tests := []struct {
		name    string
		create  redisCmd
		want    storage.RedisParameters
		wantErr bool
	}{
		{"simple", redisCmd{}, storage.RedisParameters{}, false},
		{
			"memorySize",
			redisCmd{MemorySize: "1G"},
			storage.RedisParameters{MemorySize: &storage.RedisMemorySize{Quantity: resource.MustParse("1G")}},
			false,
		},
		{
			"maxMemoryPolicy",
			redisCmd{MaxMemoryPolicy: storage.RedisMaxMemoryPolicy("noeviction")},
			storage.RedisParameters{MaxMemoryPolicy: storage.RedisMaxMemoryPolicy("noeviction")},
			false,
		},
		{
			"allowedCIDRs",
			redisCmd{AllowedCIDRs: []storage.IPv4CIDR{storage.IPv4CIDR("0.0.0.0/0")}},
			storage.RedisParameters{AllowedCIDRs: []storage.IPv4CIDR{storage.IPv4CIDR("0.0.0.0/0")}},
			false,
		},
		{
			"invalid",
			redisCmd{MemorySize: "invalid"},
			storage.RedisParameters{},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.create.Name = "test-" + t.Name()
			tt.create.Wait = false
			tt.create.WaitTimeout = time.Second

			scheme, err := api.NewScheme()
			if err != nil {
				t.Fatal(err)
			}
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			apiClient := &api.Client{WithWatch: client, Project: "default"}
			ctx := context.Background()

			if err := tt.create.Run(ctx, apiClient); (err != nil) != tt.wantErr {
				t.Errorf("redisCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			created := &storage.Redis{ObjectMeta: metav1.ObjectMeta{Name: tt.create.Name, Namespace: apiClient.Project}}
			if err := apiClient.Get(ctx, api.ObjectName(created), created); (err != nil) != tt.wantErr {
				t.Fatalf("expected redis to exist, got: %s", err)
			}
			if tt.wantErr {
				return
			}

			if !reflect.DeepEqual(created.Spec.ForProvider, tt.want) {
				t.Fatalf("expected redis.Spec.ForProvider = %v, got: %v", created.Spec.ForProvider, tt.want)
			}
		})
	}
}
