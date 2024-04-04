package create

import (
	"context"
	"reflect"
	"testing"
	"time"

	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMySQL(t *testing.T) {
	tests := []struct {
		name    string
		create  mySQLCmd
		want    storage.MySQLParameters
		wantErr bool
	}{
		{"simple", mySQLCmd{}, storage.MySQLParameters{}, false},
		{
			"machineType",
			mySQLCmd{MachineType: infra.MachineType("nine-standard-1")},
			storage.MySQLParameters{MachineType: infra.MachineType("nine-standard-1")},
			false,
		},
		{
			"sshKeys",
			mySQLCmd{SSHKeys: []storage.SSHKey{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJGG5/nnivrW4zLD4ANLclVT3y68GAg6NOA3HpzFLo5e test@test"}},
			storage.MySQLParameters{SSHKeys: []storage.SSHKey{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJGG5/nnivrW4zLD4ANLclVT3y68GAg6NOA3HpzFLo5e test@test"}},
			false,
		},
		{
			"sqlMode",
			mySQLCmd{SQLMode: &[]storage.MySQLMode{"ONLY_FULL_GROUP_BY"}},
			storage.MySQLParameters{SQLMode: &[]storage.MySQLMode{"ONLY_FULL_GROUP_BY"}},
			false,
		},
		{
			"allowedCIDRs",
			mySQLCmd{AllowedCidrs: []storage.IPv4CIDR{storage.IPv4CIDR("0.0.0.0/0")}},
			storage.MySQLParameters{AllowedCIDRs: []storage.IPv4CIDR{storage.IPv4CIDR("0.0.0.0/0")}},
			false,
		},
		{
			"characterSet",
			mySQLCmd{CharacterSetName: "utf8mb4", CharacterSetCollation: "utf8mb4_unicode_ci"},
			storage.MySQLParameters{CharacterSet: storage.MySQLCharacterSet{Name: "utf8mb4", Collation: "utf8mb4_unicode_ci"}},
			false,
		},
		{
			"longQueryTime",
			mySQLCmd{LongQueryTime: storage.LongQueryTime("300")},
			storage.MySQLParameters{LongQueryTime: storage.LongQueryTime("300")},
			false,
		},
		{
			"minWordLength",
			mySQLCmd{MinWordLength: ptr.To(5)},
			storage.MySQLParameters{MinWordLength: ptr.To(5)},
			false,
		},
		{
			"transactionIsolation",
			mySQLCmd{TransactionIsolation: storage.MySQLTransactionCharacteristic("READ-UNCOMMITTED")},
			storage.MySQLParameters{TransactionIsolation: storage.MySQLTransactionCharacteristic("READ-UNCOMMITTED")},
			false,
		},
		{
			"keepDailyBackups",
			mySQLCmd{KeepDailyBackups: ptr.To(5)},
			storage.MySQLParameters{KeepDailyBackups: ptr.To(5)},
			false,
		},
		{
			"minWordLength",
			mySQLCmd{MinWordLength: ptr.To(5)},
			storage.MySQLParameters{MinWordLength: ptr.To(5)},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.create.Name = "test-" + t.Name()
			tt.create.Wait = false
			tt.create.WaitTimeout = time.Second

			scheme, err := api.NewScheme()
			if err != nil {
				t.Fatal(err)
			}
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			apiClient := &api.Client{WithWatch: client, Project: "default"}
			ctx := context.Background()

			if err := tt.create.Run(ctx, apiClient); (err != nil) != tt.wantErr {
				t.Errorf("mySQLCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			created := &storage.MySQL{ObjectMeta: metav1.ObjectMeta{Name: tt.create.Name, Namespace: apiClient.Project}}
			if err := apiClient.Get(ctx, api.ObjectName(created), created); (err != nil) != tt.wantErr {
				t.Fatalf("expected mysql to exist, got: %s", err)
			}
			if tt.wantErr {
				return
			}

			if !reflect.DeepEqual(created.Spec.ForProvider, tt.want) {
				t.Fatalf("expected mysql.Spec.ForProvider = %v, got: %v", created.Spec.ForProvider, tt.want)
			}
		})
	}
}
