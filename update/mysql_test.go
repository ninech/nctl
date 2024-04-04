package update

import (
	"context"
	"reflect"
	"testing"

	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		{"simple", storage.MySQLParameters{}, mySQLCmd{}, storage.MySQLParameters{}, false},
		{
			"machineType",
			storage.MySQLParameters{},
			mySQLCmd{MachineType: ptr.To(infra.MachineType("nine-standard-1"))},
			storage.MySQLParameters{MachineType: infra.MachineType("nine-standard-1")},
			false,
		},
		{
			"machineType",
			storage.MySQLParameters{MachineType: infra.MachineType("nine-standard-2")},
			mySQLCmd{MachineType: ptr.To(infra.MachineType("nine-standard-1"))},
			storage.MySQLParameters{MachineType: infra.MachineType("nine-standard-1")},
			false,
		},
		{
			"sqlMode",
			storage.MySQLParameters{},
			mySQLCmd{SQLMode: ptr.To([]storage.MySQLMode{"ERROR_FOR_DIVISION_BY_ZERO"})},
			storage.MySQLParameters{SQLMode: &[]storage.MySQLMode{"ERROR_FOR_DIVISION_BY_ZERO"}},
			false,
		},
		{
			"sqlMode",
			storage.MySQLParameters{SQLMode: &[]storage.MySQLMode{"ERROR_FOR_DIVISION_BY_ZERO"}},
			mySQLCmd{SQLMode: ptr.To([]storage.MySQLMode{"ALLOW_INVALID_DATES", "STRICT_TRANS_TABLES"})},
			storage.MySQLParameters{SQLMode: &[]storage.MySQLMode{"ALLOW_INVALID_DATES", "STRICT_TRANS_TABLES"}},
			false,
		},
		{
			"sshKeys",
			storage.MySQLParameters{},
			mySQLCmd{SSHKeys: &[]storage.SSHKey{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJGG5/nnivrW4zLD4ANLclVT3y68GAg6NOA3HpzFLo5e test@test"}},
			storage.MySQLParameters{SSHKeys: []storage.SSHKey{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJGG5/nnivrW4zLD4ANLclVT3y68GAg6NOA3HpzFLo5e test@test"}},
			false,
		},
		{
			"characterSet",
			storage.MySQLParameters{},
			mySQLCmd{CharacterSetName: ptr.To("latin1"), CharacterSetCollation: ptr.To("latin1-general-ci")},
			storage.MySQLParameters{CharacterSet: storage.MySQLCharacterSet{Name: "latin1", Collation: "latin1-general-ci"}},
			false,
		},
		{
			"longQueryTime",
			storage.MySQLParameters{},
			mySQLCmd{LongQueryTime: ptr.To(storage.LongQueryTime("300"))},
			storage.MySQLParameters{LongQueryTime: storage.LongQueryTime("300")},
			false,
		},
		{
			"minWordLength",
			storage.MySQLParameters{},
			mySQLCmd{MinWordLength: ptr.To(5)},
			storage.MySQLParameters{MinWordLength: ptr.To(5)},
			false,
		},
		{
			"transactionIsolation",
			storage.MySQLParameters{},
			mySQLCmd{TransactionIsolation: ptr.To(storage.MySQLTransactionCharacteristic("READ-UNCOMMITTED"))},
			storage.MySQLParameters{TransactionIsolation: storage.MySQLTransactionCharacteristic("READ-UNCOMMITTED")},
			false,
		},
		{
			"keepDailyBackups",
			storage.MySQLParameters{},
			mySQLCmd{KeepDailyBackups: ptr.To(5)},
			storage.MySQLParameters{KeepDailyBackups: ptr.To(5)},
			false,
		},
		{
			"allowedCIDRs",
			storage.MySQLParameters{},
			mySQLCmd{AllowedCidrs: &[]storage.IPv4CIDR{storage.IPv4CIDR("0.0.0.0/0")}},
			storage.MySQLParameters{AllowedCIDRs: []storage.IPv4CIDR{storage.IPv4CIDR("0.0.0.0/0")}},
			false,
		},
		{
			"allowedCIDRs",
			storage.MySQLParameters{AllowedCIDRs: []storage.IPv4CIDR{"192.168.0.1/24"}},
			mySQLCmd{AllowedCidrs: &[]storage.IPv4CIDR{storage.IPv4CIDR("0.0.0.0/0")}},
			storage.MySQLParameters{AllowedCIDRs: []storage.IPv4CIDR{storage.IPv4CIDR("0.0.0.0/0")}},
			false,
		},
		{
			"allowedCIDRs",
			storage.MySQLParameters{AllowedCIDRs: []storage.IPv4CIDR{"0.0.0.0/0"}},
			mySQLCmd{MachineType: ptr.To(infra.MachineType("nine-standard-1"))},
			storage.MySQLParameters{MachineType: infra.MachineType("nine-standard-1"), AllowedCIDRs: []storage.IPv4CIDR{storage.IPv4CIDR("0.0.0.0/0")}},
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

			created := test.MySQL(tt.update.Name, apiClient.Project, "nine-es34")
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(ctx, created); err != nil {
				t.Fatalf("mysql create error, got: %s", err)
			}
			if err := apiClient.Get(ctx, api.ObjectName(created), created); err != nil {
				t.Fatalf("expected mysql to exist, got: %s", err)
			}

			updated := &storage.MySQL{ObjectMeta: metav1.ObjectMeta{Name: created.Name, Namespace: created.Namespace}}
			if err := tt.update.Run(ctx, apiClient); (err != nil) != tt.wantErr {
				t.Errorf("mySQLCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := apiClient.Get(ctx, api.ObjectName(updated), updated); err != nil {
				t.Fatalf("expected mysql to exist, got: %s", err)
			}

			if !reflect.DeepEqual(updated.Spec.ForProvider, tt.want) {
				t.Fatalf("expected mysql.Spec.ForProvider = %v, got: %v", updated.Spec.ForProvider, tt.want)
			}
		})
	}
}
