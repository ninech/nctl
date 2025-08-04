package update

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func TestOpenSearch(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		create  storage.OpenSearchParameters
		update  openSearchCmd
		want    storage.OpenSearchParameters
		wantErr bool
	}{
		{
			name: "simple",
		},
		{
			name:   "increase-machineType",
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.update.Name = "test-" + t.Name()

			apiClient, err := test.SetupClient()
			require.NoError(t, err)

			created := test.OpenSearch(tt.update.Name, apiClient.Project, "nine-es34")
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(ctx, created); err != nil {
				t.Fatalf("opensearch create error, got: %s", err)
			}
			if err := apiClient.Get(ctx, api.ObjectName(created), created); err != nil {
				t.Fatalf("expected opensearch to exist, got: %s", err)
			}

			updated := &storage.OpenSearch{}
			if err := tt.update.Run(ctx, apiClient); (err != nil) != tt.wantErr {
				t.Errorf("openSearchCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := apiClient.Get(ctx, api.ObjectName(created), updated); err != nil {
				t.Fatalf("expected openSearch to exist, got: %s", err)
			}

			if !cmp.Equal(updated.Spec.ForProvider, tt.want) {
				t.Fatalf("expected openSearch.Spec.ForProvider = %v, got: %v", updated.Spec.ForProvider, tt.want)
			}
		})
	}
}
