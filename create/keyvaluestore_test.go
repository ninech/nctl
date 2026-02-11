package create

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestKeyValueStore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		create  keyValueStoreCmd
		want    storage.KeyValueStoreParameters
		wantErr bool
	}{
		{
			name: "simple",
		},
		{
			name:   "memorySize",
			create: keyValueStoreCmd{MemorySize: ptr.To(storage.KeyValueStoreMemorySize{Quantity: resource.MustParse("1G")})},
			want: storage.KeyValueStoreParameters{
				MemorySize: &storage.KeyValueStoreMemorySize{
					Quantity: resource.MustParse("1G"),
				},
			},
		},
		{
			name: "maxMemoryPolicy",
			create: keyValueStoreCmd{
				MaxMemoryPolicy: storage.KeyValueStoreMaxMemoryPolicy("noeviction"),
			},
			want: storage.KeyValueStoreParameters{
				MaxMemoryPolicy: storage.KeyValueStoreMaxMemoryPolicy("noeviction"),
			},
		},
		{
			name: "allowedCIDRs",
			create: keyValueStoreCmd{
				AllowedCidrs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")},
			},
			want: storage.KeyValueStoreParameters{
				AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")},
			},
		},
		{
			name: "publicNetworking-deprecated",
			create: keyValueStoreCmd{
				PublicNetworkingEnabled: ptr.To(true),
			},
			want: storage.KeyValueStoreParameters{
				PublicNetworkingEnabled: ptr.To(true),
			},
		},
		{
			name: "publicNetworking",
			create: keyValueStoreCmd{
				PublicNetworking: ptr.To(true),
			},
			want: storage.KeyValueStoreParameters{
				PublicNetworkingEnabled: ptr.To(true),
			},
		},
		{
			name: "publicNetworking-disabled-deprecated",
			create: keyValueStoreCmd{
				PublicNetworkingEnabled: ptr.To(false),
			},
			want: storage.KeyValueStoreParameters{
				PublicNetworkingEnabled: ptr.To(false),
			},
		},
		{
			name: "publicNetworking-disabled",
			create: keyValueStoreCmd{
				PublicNetworking: ptr.To(false),
			},
			want: storage.KeyValueStoreParameters{
				PublicNetworkingEnabled: ptr.To(false),
			},
		},
		{
			name: "publicNetworking-disabled-both",
			create: keyValueStoreCmd{
				PublicNetworking:        ptr.To(false),
				PublicNetworkingEnabled: ptr.To(true),
			},
			want: storage.KeyValueStoreParameters{
				PublicNetworkingEnabled: ptr.To(false),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			tt.create.Name = "test-" + t.Name()
			tt.create.Wait = false
			tt.create.WaitTimeout = time.Second

			apiClient, err := test.SetupClient()
			is.NoError(err)

			if err := tt.create.Run(t.Context(), apiClient); (err != nil) != tt.wantErr {
				t.Errorf("keyValueStoreCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			created := &storage.KeyValueStore{ObjectMeta: metav1.ObjectMeta{Name: tt.create.Name, Namespace: apiClient.Project}}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), created); (err != nil) != tt.wantErr {
				t.Fatalf("expected keyvaluestore to exist, got: %s", err)
			}
			if tt.wantErr {
				return
			}

			is.True(cmp.Equal(tt.want, created.Spec.ForProvider))
		})
	}
}
