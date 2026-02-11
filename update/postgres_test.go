package update

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"k8s.io/utils/ptr"
)

func TestPostgres(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		create  storage.PostgresParameters
		update  postgresCmd
		want    storage.PostgresParameters
		wantErr bool
	}{
		{
			name: "simple",
		},
		{
			name:   "increase-machineType",
			update: postgresCmd{MachineType: ptr.To(infra.MachineTypeNineDBS.String())},
			want:   storage.PostgresParameters{MachineType: infra.MachineTypeNineDBS},
		},
		{
			name:   "decrease-machineType",
			create: storage.PostgresParameters{MachineType: infra.MachineTypeNineDBM},
			update: postgresCmd{MachineType: ptr.To(infra.MachineTypeNineDBS.String())},
			want:   storage.PostgresParameters{MachineType: infra.MachineTypeNineDBS},
		},
		{
			name: "sshKeys",
			update: postgresCmd{
				SSHKeys: []storage.SSHKey{
					"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJGG5/nnivrW4zLD4ANLclVT3y68GAg6NOA3HpzFLo5e test@test",
				},
			},
			want: storage.PostgresParameters{
				SSHKeys: []storage.SSHKey{
					"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJGG5/nnivrW4zLD4ANLclVT3y68GAg6NOA3HpzFLo5e test@test",
				},
			},
		},
		{
			name:   "keepDailyBackups",
			update: postgresCmd{KeepDailyBackups: ptr.To(5)},
			want:   storage.PostgresParameters{KeepDailyBackups: ptr.To(5)},
		},
		{
			name:   "allowedCIDRs-nothing-set-initially",
			update: postgresCmd{AllowedCidrs: &[]meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
			want:   storage.PostgresParameters{AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
		},
		{
			name:   "allowedCIDRs-set-initially",
			create: storage.PostgresParameters{AllowedCIDRs: []meta.IPv4CIDR{"192.168.0.1/24"}},
			update: postgresCmd{AllowedCidrs: &[]meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
			want:   storage.PostgresParameters{AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
		},
		{
			name:   "multi-update",
			create: storage.PostgresParameters{AllowedCIDRs: []meta.IPv4CIDR{"0.0.0.0/0"}},
			update: postgresCmd{MachineType: ptr.To(infra.MachineTypeNineDBS.String())},
			want: storage.PostgresParameters{
				MachineType:  infra.MachineTypeNineDBS,
				AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.update.Name = "test-" + t.Name()

			apiClient := test.SetupClient(t)

			created := test.Postgres(tt.update.Name, apiClient.Project, "nine-es34")
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(t.Context(), created); err != nil {
				t.Fatalf("postgres create error, got: %s", err)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), created); err != nil {
				t.Fatalf("expected postgres to exist, got: %s", err)
			}

			updated := &storage.Postgres{}
			if err := tt.update.Run(t.Context(), apiClient); (err != nil) != tt.wantErr {
				t.Errorf("postgresCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), updated); err != nil {
				t.Fatalf("expected postgres to exist, got: %s", err)
			}

			if !cmp.Equal(updated.Spec.ForProvider, tt.want) {
				t.Fatalf("expected postgres.Spec.ForProvider = %v, got: %v", updated.Spec.ForProvider, tt.want)
			}
		})
	}
}
