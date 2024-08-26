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

func TestMySQL(t *testing.T) {
	tests := []struct {
		name    string
		create  storage.MySQLParameters
		update  mySQLCmd
		want    storage.MySQLParameters
		wantErr bool
	}{
		{
			name: "simple",
		},
		{
			name:   "increase-machineType",
			update: mySQLCmd{MachineType: ptr.To(infra.MachineType("nine-db-prod-s"))},
			want:   storage.MySQLParameters{MachineType: infra.MachineType("nine-db-prod-s")},
		},
		{
			name:   "decrease-machineType",
			create: storage.MySQLParameters{MachineType: infra.MachineType("nine-db-prod-m")},
			update: mySQLCmd{MachineType: ptr.To(infra.MachineType("nine-db-prod-s"))},
			want:   storage.MySQLParameters{MachineType: infra.MachineType("nine-db-prod-s")},
		},
		{
			name:   "sqlMode-no-mode-set-initially",
			update: mySQLCmd{SQLMode: ptr.To([]storage.MySQLMode{"ERROR_FOR_DIVISION_BY_ZERO"})},
			want:   storage.MySQLParameters{SQLMode: &[]storage.MySQLMode{"ERROR_FOR_DIVISION_BY_ZERO"}},
		},
		{
			name:   "sqlMode-initially-set",
			create: storage.MySQLParameters{SQLMode: &[]storage.MySQLMode{"ERROR_FOR_DIVISION_BY_ZERO"}},
			update: mySQLCmd{SQLMode: ptr.To([]storage.MySQLMode{"ALLOW_INVALID_DATES", "STRICT_TRANS_TABLES"})},
			want:   storage.MySQLParameters{SQLMode: &[]storage.MySQLMode{"ALLOW_INVALID_DATES", "STRICT_TRANS_TABLES"}},
		},
		{
			name:   "sshKeys",
			update: mySQLCmd{SSHKeys: []storage.SSHKey{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJGG5/nnivrW4zLD4ANLclVT3y68GAg6NOA3HpzFLo5e test@test"}},
			want:   storage.MySQLParameters{SSHKeys: []storage.SSHKey{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJGG5/nnivrW4zLD4ANLclVT3y68GAg6NOA3HpzFLo5e test@test"}},
		},
		{
			name:   "characterSet",
			update: mySQLCmd{CharacterSetName: ptr.To("latin1"), CharacterSetCollation: ptr.To("latin1-general-ci")},
			want:   storage.MySQLParameters{CharacterSet: storage.MySQLCharacterSet{Name: "latin1", Collation: "latin1-general-ci"}},
		},
		{
			name:   "longQueryTime",
			update: mySQLCmd{LongQueryTime: ptr.To(storage.LongQueryTime("300"))},
			want:   storage.MySQLParameters{LongQueryTime: storage.LongQueryTime("300")},
		},
		{
			name:   "minWordLength",
			update: mySQLCmd{MinWordLength: ptr.To(5)},
			want:   storage.MySQLParameters{MinWordLength: ptr.To(5)},
		},
		{
			name:   "transactionIsolation",
			update: mySQLCmd{TransactionIsolation: ptr.To(storage.MySQLTransactionCharacteristic("READ-UNCOMMITTED"))},
			want:   storage.MySQLParameters{TransactionIsolation: storage.MySQLTransactionCharacteristic("READ-UNCOMMITTED")},
		},
		{
			name:   "keepDailyBackups",
			update: mySQLCmd{KeepDailyBackups: ptr.To(5)},
			want:   storage.MySQLParameters{KeepDailyBackups: ptr.To(5)},
		},
		{
			name:   "allowedCIDRs-nothing-set-initially",
			update: mySQLCmd{AllowedCidrs: &[]meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
			want:   storage.MySQLParameters{AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
		},
		{
			name:   "allowedCIDRs-set-initially",
			create: storage.MySQLParameters{AllowedCIDRs: []meta.IPv4CIDR{"192.168.0.1/24"}},
			update: mySQLCmd{AllowedCidrs: &[]meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
			want:   storage.MySQLParameters{AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
		},
		{
			name:   "multi-update",
			create: storage.MySQLParameters{AllowedCIDRs: []meta.IPv4CIDR{"0.0.0.0/0"}},
			update: mySQLCmd{MachineType: ptr.To(infra.MachineType("nine-db-prod-s"))},
			want:   storage.MySQLParameters{MachineType: infra.MachineType("nine-db-prod-s"), AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
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

			created := test.MySQL(tt.update.Name, apiClient.Project, "nine-es34")
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(ctx, created); err != nil {
				t.Fatalf("mysql create error, got: %s", err)
			}
			if err := apiClient.Get(ctx, api.ObjectName(created), created); err != nil {
				t.Fatalf("expected mysql to exist, got: %s", err)
			}

			updated := &storage.MySQL{}
			if err := tt.update.Run(ctx, apiClient); (err != nil) != tt.wantErr {
				t.Errorf("mySQLCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := apiClient.Get(ctx, api.ObjectName(created), updated); err != nil {
				t.Fatalf("expected mysql to exist, got: %s", err)
			}

			if !reflect.DeepEqual(updated.Spec.ForProvider, tt.want) {
				t.Fatalf("expected mysql.Spec.ForProvider = %v, got: %v", updated.Spec.ForProvider, tt.want)
			}
		})
	}
}
