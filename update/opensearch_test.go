package update

import (
	"testing"

	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/create"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func TestOpenSearch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		create  storage.OpenSearchParameters
		update  openSearchCmd
		want    storage.OpenSearchParameters
		wantErr bool
	}{
		{
			name:   "increase-machineType",
			create: storage.OpenSearchParameters{MachineType: infra.MachineTypeNineSearchS},
			update: openSearchCmd{MachineType: ptr.To(infra.MachineTypeNineSearchL.String())},
			want:   storage.OpenSearchParameters{MachineType: infra.MachineTypeNineSearchL},
		},
		{
			name:   "decrease-machineType",
			create: storage.OpenSearchParameters{MachineType: infra.MachineTypeNineSearchL},
			update: openSearchCmd{MachineType: ptr.To(infra.MachineTypeNineSearchM.String())},
			want:   storage.OpenSearchParameters{MachineType: infra.MachineTypeNineSearchM},
		},
		{
			name:   "allowedCIDRs-nothing-set-initially",
			update: openSearchCmd{AllowedCidrs: &[]meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
			want:   storage.OpenSearchParameters{AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
		},
		{
			name:   "allowedCIDRs-set-initially",
			create: storage.OpenSearchParameters{AllowedCIDRs: []meta.IPv4CIDR{"192.168.0.1/24"}},
			update: openSearchCmd{AllowedCidrs: &[]meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
			want:   storage.OpenSearchParameters{AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
		},
		{
			name:   "multi-update",
			create: storage.OpenSearchParameters{AllowedCIDRs: []meta.IPv4CIDR{"0.0.0.0/0"}},
			update: openSearchCmd{MachineType: ptr.To(infra.MachineTypeNineSearchM.String())},
			want: storage.OpenSearchParameters{
				MachineType:  infra.MachineTypeNineSearchM,
				AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")},
			},
		},
		{
			name: "bucket-users-set",
			update: openSearchCmd{BucketUsers: &[]create.LocalReference{
				{LocalReference: meta.LocalReference{Name: "user1"}},
				{LocalReference: meta.LocalReference{Name: "user2"}},
			}},
			want: storage.OpenSearchParameters{
				BucketUsers: []meta.LocalReference{{Name: "user1"}, {Name: "user2"}},
			},
		},
		{
			name: "disable-public-networking-deprecated",
			create: storage.OpenSearchParameters{
				PublicNetworkingEnabled: ptr.To(true),
			},
			update: openSearchCmd{PublicNetworkingEnabled: ptr.To(false)},
			want: storage.OpenSearchParameters{
				PublicNetworkingEnabled: ptr.To(false),
			},
		},
		{
			name: "disable-public-networking",
			create: storage.OpenSearchParameters{
				PublicNetworkingEnabled: ptr.To(true),
			},
			update: openSearchCmd{PublicNetworking: ptr.To(false)},
			want: storage.OpenSearchParameters{
				PublicNetworkingEnabled: ptr.To(false),
			},
		},
		{
			name: "disable-public-networking-both",
			create: storage.OpenSearchParameters{
				PublicNetworkingEnabled: ptr.To(true),
			},
			update: openSearchCmd{PublicNetworking: ptr.To(false), PublicNetworkingEnabled: ptr.To(true)},
			want: storage.OpenSearchParameters{
				PublicNetworkingEnabled: ptr.To(false),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			is := require.New(t)

			tt.update.Name = "test-" + t.Name()

			apiClient := test.SetupClient(t)

			created := test.OpenSearch(tt.update.Name, apiClient.Project, meta.LocationNineES34)
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(t.Context(), created); err != nil {
				t.Fatalf("opensearch create error, got: %s", err)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), created); err != nil {
				t.Fatalf("expected opensearch to exist, got: %s", err)
			}

			updated := &storage.OpenSearch{}
			if err := tt.update.Run(t.Context(), apiClient); (err != nil) != tt.wantErr {
				t.Errorf("openSearchCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), updated); err != nil {
				t.Fatalf("expected openSearch to exist, got: %s", err)
			}

			is.EqualExportedValues(tt.want, updated.Spec.ForProvider) // As machine types contain unexported values.
		})
	}
}
