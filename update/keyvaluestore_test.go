package update

import (
	"context"
	"reflect"
	"testing"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"
)

func TestKeyValueStore(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		create  storage.KeyValueStoreParameters
		update  keyValueStoreCmd
		want    storage.KeyValueStoreParameters
		wantErr bool
	}{
		{
			name: "simple",
		},
		{
			name:   "memorySize upgrade",
			update: keyValueStoreCmd{MemorySize: ptr.To("1G")},
			want:   storage.KeyValueStoreParameters{MemorySize: memorySize("1G")},
		},
		{
			name:   "memorySize downgrade",
			create: storage.KeyValueStoreParameters{MemorySize: memorySize("2G")},
			update: keyValueStoreCmd{MemorySize: ptr.To("1G")},
			want:   storage.KeyValueStoreParameters{MemorySize: memorySize("1G")},
		},
		{
			name:    "invalid",
			create:  storage.KeyValueStoreParameters{MemorySize: memorySize("2G")},
			update:  keyValueStoreCmd{MemorySize: ptr.To("invalid")},
			want:    storage.KeyValueStoreParameters{MemorySize: memorySize("2G")},
			wantErr: true,
		},
		{
			name: "maxMemoryPolicy-to-noeviction",
			update: keyValueStoreCmd{
				MaxMemoryPolicy: ptr.To(storage.KeyValueStoreMaxMemoryPolicy("noeviction")),
			},
			want: storage.KeyValueStoreParameters{
				MaxMemoryPolicy: storage.KeyValueStoreMaxMemoryPolicy("noeviction"),
			},
		},
		{
			name: "maxMemoryPolicy-from-allkeys-lfu-to-noeviction",
			create: storage.KeyValueStoreParameters{
				MaxMemoryPolicy: storage.KeyValueStoreMaxMemoryPolicy("allkeys-lfu"),
			},
			update: keyValueStoreCmd{
				MaxMemoryPolicy: ptr.To(storage.KeyValueStoreMaxMemoryPolicy("noeviction")),
			},
			want: storage.KeyValueStoreParameters{
				MaxMemoryPolicy: storage.KeyValueStoreMaxMemoryPolicy("noeviction"),
			},
		},
		{
			name: "allowedCIDRs",
			update: keyValueStoreCmd{
				AllowedCidrs: &[]meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")},
			},
			want: storage.KeyValueStoreParameters{
				AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")},
			},
		},
		{
			name: "allowedCIDRs-from-local-to-all-allowed",
			create: storage.KeyValueStoreParameters{
				AllowedCIDRs: []meta.IPv4CIDR{"192.168.0.1/24"},
			},
			update: keyValueStoreCmd{
				AllowedCidrs: &[]meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")},
			},
			want: storage.KeyValueStoreParameters{
				AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")},
			},
		},
		{
			name: "update-memory-size-when-allowedCIDRs-are-set",
			create: storage.KeyValueStoreParameters{
				AllowedCIDRs: []meta.IPv4CIDR{"0.0.0.0/0"},
			},
			update: keyValueStoreCmd{MemorySize: ptr.To("1G")},
			want: storage.KeyValueStoreParameters{
				MemorySize:   memorySize("1G"),
				AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")},
			},
		},
		{
			name: "update-public-networking",
			create: storage.KeyValueStoreParameters{
				PublicNetworkingEnabled: ptr.To(true),
			},
			update: keyValueStoreCmd{PublicNetworkingEnabled: ptr.To(false)},
			want: storage.KeyValueStoreParameters{
				PublicNetworkingEnabled: ptr.To(false),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.update.Name = "test-" + t.Name()

			apiClient, err := test.SetupClient()
			require.NoError(t, err)

			created := test.KeyValueStore(tt.update.Name, apiClient.Project, "nine-es34")
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(ctx, created); err != nil {
				t.Fatalf("keyvaluestore create error, got: %s", err)
			}
			if err := apiClient.Get(ctx, api.ObjectName(created), created); err != nil {
				t.Fatalf("expected keyvaluestore to exist, got: %s", err)
			}

			updated := &storage.KeyValueStore{}
			if err := tt.update.Run(ctx, apiClient); (err != nil) != tt.wantErr {
				t.Errorf("keyValueStoreCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := apiClient.Get(ctx, api.ObjectName(created), updated); err != nil {
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
