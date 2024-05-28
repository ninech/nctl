package update

import (
	"context"
	"reflect"
	"testing"

	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPostgres(t *testing.T) {
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
			update: postgresCmd{MachineType: ptr.To(infra.MachineType("nine-standard-1"))},
			want:   storage.PostgresParameters{MachineType: infra.MachineType("nine-standard-1")},
		},
		{
			name:   "decrease-machineType",
			create: storage.PostgresParameters{MachineType: infra.MachineType("nine-standard-2")},
			update: postgresCmd{MachineType: ptr.To(infra.MachineType("nine-standard-1"))},
			want:   storage.PostgresParameters{MachineType: infra.MachineType("nine-standard-1")},
		},
		{
			name:   "sshKeys",
			update: postgresCmd{SSHKeys: []storage.SSHKey{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJGG5/nnivrW4zLD4ANLclVT3y68GAg6NOA3HpzFLo5e test@test"}},
			want:   storage.PostgresParameters{SSHKeys: []storage.SSHKey{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJGG5/nnivrW4zLD4ANLclVT3y68GAg6NOA3HpzFLo5e test@test"}},
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
			update: postgresCmd{MachineType: ptr.To(infra.MachineType("nine-standard-1"))},
			want:   storage.PostgresParameters{MachineType: infra.MachineType("nine-standard-1"), AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
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

			created := test.Postgres(tt.update.Name, apiClient.Project, "nine-es34")
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(ctx, created); err != nil {
				t.Fatalf("postgres create error, got: %s", err)
			}
			if err := apiClient.Get(ctx, api.ObjectName(created), created); err != nil {
				t.Fatalf("expected postgres to exist, got: %s", err)
			}

			updated := &storage.Postgres{}
			if err := tt.update.Run(ctx, apiClient); (err != nil) != tt.wantErr {
				t.Errorf("postgresCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := apiClient.Get(ctx, api.ObjectName(created), updated); err != nil {
				t.Fatalf("expected postgres to exist, got: %s", err)
			}

			if !reflect.DeepEqual(updated.Spec.ForProvider, tt.want) {
				t.Fatalf("expected postgres.Spec.ForProvider = %v, got: %v", updated.Spec.ForProvider, tt.want)
			}
		})
	}
}
