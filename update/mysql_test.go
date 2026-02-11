package update

import (
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

func TestMySQL(t *testing.T) {
	t.Parallel()

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
			update: mySQLCmd{MachineType: ptr.To(infra.MachineTypeNineDBM.String())},
			want:   storage.MySQLParameters{MachineType: infra.MachineTypeNineDBM},
		},
		{
			name:   "decrease-machineType",
			create: storage.MySQLParameters{MachineType: infra.MachineTypeNineDBM},
			update: mySQLCmd{MachineType: ptr.To(infra.MachineTypeNineDBS.String())},
			want:   storage.MySQLParameters{MachineType: infra.MachineTypeNineDBS},
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
			update: mySQLCmd{MachineType: ptr.To(infra.MachineTypeNineDBS.String())},
			want:   storage.MySQLParameters{MachineType: infra.MachineTypeNineDBS, AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			is := require.New(t)

			tt.update.Name = "test-" + t.Name()

			apiClient, err := test.SetupClient()
			is.NoError(err)

			created := test.MySQL(tt.update.Name, apiClient.Project, "nine-es34")
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(t.Context(), created); err != nil {
				t.Fatalf("mysql create error, got: %s", err)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), created); err != nil {
				t.Fatalf("expected mysql to exist, got: %s", err)
			}

			updated := &storage.MySQL{}
			if err := tt.update.Run(t.Context(), apiClient); (err != nil) != tt.wantErr {
				t.Errorf("mySQLCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), updated); err != nil {
				t.Fatalf("expected mysql to exist, got: %s", err)
			}

			if !cmp.Equal(updated.Spec.ForProvider, tt.want) {
				t.Fatalf("expected mysql.Spec.ForProvider = %v, got: %v", updated.Spec.ForProvider, tt.want)
			}
		})
	}
}
