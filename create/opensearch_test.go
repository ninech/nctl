package create

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestOpenSearch(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		create  openSearchCmd
		want    storage.OpenSearchParameters
		wantErr bool
	}{
		{
			name: "default",
			want: storage.OpenSearchParameters{},
		},
		{
			name:   "customLocation",
			create: openSearchCmd{Location: "nine-cz41"},
			want: storage.OpenSearchParameters{
				Location: meta.LocationNineCZ41,
			},
		},
		{
			name:   "multiClusterType",
			create: openSearchCmd{ClusterType: storage.OpenSearchClusterTypeMulti},
			want: storage.OpenSearchParameters{
				ClusterType: storage.OpenSearchClusterTypeMulti,
			},
		},
		{
			name:   "customMachineType",
			create: openSearchCmd{MachineType: infra.MachineTypeNineSearchL.String()},
			want: storage.OpenSearchParameters{
				MachineType: infra.MachineTypeNineSearchL,
			},
		},
		{
			name: "allowedCIDRs",
			create: openSearchCmd{
				AllowedCidrs: []meta.IPv4CIDR{meta.IPv4CIDR("192.168.1.0/24")},
			},
			want: storage.OpenSearchParameters{
				AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("192.168.1.0/24")},
			},
		},
		{
			name: "publicNetworking-deprecated",
			create: openSearchCmd{
				PublicNetworkingEnabled: ptr.To(true),
			},
			want: storage.OpenSearchParameters{
				PublicNetworkingEnabled: ptr.To(true),
			},
		},
		{
			name: "publicNetworking",
			create: openSearchCmd{
				PublicNetworking: ptr.To(true),
			},
			want: storage.OpenSearchParameters{
				PublicNetworkingEnabled: ptr.To(true),
			},
		},
		{
			name: "publicNetworking-disabled-deprecated",
			create: openSearchCmd{
				PublicNetworkingEnabled: ptr.To(false),
			},
			want: storage.OpenSearchParameters{
				PublicNetworkingEnabled: ptr.To(false),
			},
		},
		{
			name: "publicNetworking-disabled",
			create: openSearchCmd{
				PublicNetworking: ptr.To(false),
			},
			want: storage.OpenSearchParameters{
				PublicNetworkingEnabled: ptr.To(false),
			},
		},
		{
			name: "publicNetworking-disabled-both",
			create: openSearchCmd{
				PublicNetworking:        ptr.To(false),
				PublicNetworkingEnabled: ptr.To(true),
			},
			want: storage.OpenSearchParameters{
				PublicNetworkingEnabled: ptr.To(false),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.create.Name = "test-" + t.Name()
			tt.create.Wait = false
			tt.create.WaitTimeout = time.Second

			apiClient, err := test.SetupClient()
			require.NoError(t, err)

			if err := tt.create.Run(ctx, apiClient); (err != nil) != tt.wantErr {
				t.Errorf("openSearchCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			created := &storage.OpenSearch{ObjectMeta: metav1.ObjectMeta{Name: tt.create.Name, Namespace: apiClient.Project}}
			if err := apiClient.Get(ctx, api.ObjectName(created), created); (err != nil) != tt.wantErr {
				t.Fatalf("expected opensearch to exist, got: %s", err)
			}
			if tt.wantErr {
				return
			}

			require.True(t, cmp.Equal(tt.want, created.Spec.ForProvider))
		})
	}
}
