package update

import (
	"context"
	"reflect"
	"testing"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestKeyValueStore(t *testing.T) {
	tests := []struct {
		name    string
		create  storage.KeyValueStoreParameters
		update  keyValueStoreCmd
		want    storage.KeyValueStoreParameters
		wantErr bool
	}{
		{"simple", storage.KeyValueStoreParameters{}, keyValueStoreCmd{}, storage.KeyValueStoreParameters{}, false},
		{
			"memorySize",
			storage.KeyValueStoreParameters{},
			keyValueStoreCmd{MemorySize: ptr.To("1G")},
			storage.KeyValueStoreParameters{MemorySize: memorySize("1G")},
			false,
		},
		{
			"memorySize",
			storage.KeyValueStoreParameters{MemorySize: memorySize("2G")},
			keyValueStoreCmd{MemorySize: ptr.To("1G")},
			storage.KeyValueStoreParameters{MemorySize: memorySize("1G")},
			false,
		},
		{
			"invalid",
			storage.KeyValueStoreParameters{MemorySize: memorySize("2G")},
			keyValueStoreCmd{MemorySize: ptr.To("invalid")},
			storage.KeyValueStoreParameters{MemorySize: memorySize("2G")},
			true,
		},
		{
			"maxMemoryPolicy",
			storage.KeyValueStoreParameters{},
			keyValueStoreCmd{MaxMemoryPolicy: ptr.To(storage.KeyValueStoreMaxMemoryPolicy("noeviction"))},
			storage.KeyValueStoreParameters{MaxMemoryPolicy: storage.KeyValueStoreMaxMemoryPolicy("noeviction")},
			false,
		},
		{
			"maxMemoryPolicy",
			storage.KeyValueStoreParameters{MaxMemoryPolicy: storage.KeyValueStoreMaxMemoryPolicy("allkeys-lfu")},
			keyValueStoreCmd{MaxMemoryPolicy: ptr.To(storage.KeyValueStoreMaxMemoryPolicy("noeviction"))},
			storage.KeyValueStoreParameters{MaxMemoryPolicy: storage.KeyValueStoreMaxMemoryPolicy("noeviction")},
			false,
		},
		{
			"allowedCIDRs",
			storage.KeyValueStoreParameters{},
			keyValueStoreCmd{AllowedCidrs: &[]meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
			storage.KeyValueStoreParameters{AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
			false,
		},
		{
			"allowedCIDRs",
			storage.KeyValueStoreParameters{AllowedCIDRs: []meta.IPv4CIDR{"192.168.0.1/24"}},
			keyValueStoreCmd{AllowedCidrs: &[]meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
			storage.KeyValueStoreParameters{AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
			false,
		},
		{
			"allowedCIDRs",
			storage.KeyValueStoreParameters{AllowedCIDRs: []meta.IPv4CIDR{"0.0.0.0/0"}},
			keyValueStoreCmd{MemorySize: ptr.To("1G")},
			storage.KeyValueStoreParameters{MemorySize: memorySize("1G"), AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
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

			created := test.KeyValueStore(tt.update.Name, apiClient.Project, "nine-es34")
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(ctx, created); err != nil {
				t.Fatalf("keyvaluestore create error, got: %s", err)
			}
			if err := apiClient.Get(ctx, api.ObjectName(created), created); err != nil {
				t.Fatalf("expected keyvaluestore to exist, got: %s", err)
			}

			updated := &storage.KeyValueStore{ObjectMeta: metav1.ObjectMeta{Name: created.Name, Namespace: created.Namespace}}
			if err := tt.update.Run(ctx, apiClient); (err != nil) != tt.wantErr {
				t.Errorf("keyValueStoreCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := apiClient.Get(ctx, api.ObjectName(updated), updated); err != nil {
				t.Fatalf("expected keyvaluestore to exist, got: %s", err)
			}

			if !reflect.DeepEqual(updated.Spec.ForProvider, tt.want) {
				t.Fatalf("expected KeyValueStore.Spec.ForProvider = %v, got: %v", updated.Spec.ForProvider, tt.want)
			}
		})
	}
}

func memorySize(s string) *storage.KeyValueStoreMemorySize {
	return &storage.KeyValueStoreMemorySize{Quantity: resource.MustParse(s)}
}
