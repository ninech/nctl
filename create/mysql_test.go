package create

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestMySQL(t *testing.T) {
	tests := []struct {
		name             string
		create           mySQLCmd
		want             storage.MySQLParameters
		wantErr          bool
		interceptorFuncs *interceptor.Funcs
	}{
		{
			name:   "simple",
			create: mySQLCmd{SSHKeys: []storage.SSHKey{""}, AllowedCidrs: []storage.IPv4CIDR{}},
			want:   storage.MySQLParameters{SSHKeys: []storage.SSHKey{""}, AllowedCIDRs: []storage.IPv4CIDR{}},
		},
		{
			name:    "simpleErrorOnCreation",
			create:  mySQLCmd{},
			wantErr: true,
			interceptorFuncs: &interceptor.Funcs{
				Create: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.CreateOption) error {
					return errors.New("error on creation")
				},
			},
		},
		{
			name:   "machineType",
			create: mySQLCmd{MachineType: infra.MachineType("nine-standard-1"), SSHKeys: []storage.SSHKey{""}, AllowedCidrs: []storage.IPv4CIDR{}},
			want:   storage.MySQLParameters{MachineType: infra.MachineType("nine-standard-1"), SSHKeys: []storage.SSHKey{""}, AllowedCIDRs: []storage.IPv4CIDR{}},
		},
		{
			name:   "sshKeys",
			create: mySQLCmd{SSHKeys: []storage.SSHKey{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJGG5/nnivrW4zLD4ANLclVT3y68GAg6NOA3HpzFLo5e test@test"}, AllowedCidrs: []storage.IPv4CIDR{}},
			want:   storage.MySQLParameters{SSHKeys: []storage.SSHKey{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJGG5/nnivrW4zLD4ANLclVT3y68GAg6NOA3HpzFLo5e test@test"}, AllowedCIDRs: []storage.IPv4CIDR{}},
		},
		{
			name:   "sqlMode",
			create: mySQLCmd{SQLMode: &[]storage.MySQLMode{"ONLY_FULL_GROUP_BY"}, SSHKeys: []storage.SSHKey{""}, AllowedCidrs: []storage.IPv4CIDR{}},
			want:   storage.MySQLParameters{SQLMode: &[]storage.MySQLMode{"ONLY_FULL_GROUP_BY"}, SSHKeys: []storage.SSHKey{""}, AllowedCIDRs: []storage.IPv4CIDR{}},
		},
		{
			name:   "allowedCIDRs",
			create: mySQLCmd{AllowedCidrs: []storage.IPv4CIDR{storage.IPv4CIDR("0.0.0.0/0")}, SSHKeys: []storage.SSHKey{""}},
			want:   storage.MySQLParameters{AllowedCIDRs: []storage.IPv4CIDR{storage.IPv4CIDR("0.0.0.0/0")}, SSHKeys: []storage.SSHKey{""}},
		},
		{
			name:   "characterSet",
			create: mySQLCmd{CharacterSetName: "utf8mb4", CharacterSetCollation: "utf8mb4_unicode_ci", SSHKeys: []storage.SSHKey{""}, AllowedCidrs: []storage.IPv4CIDR{}},
			want:   storage.MySQLParameters{CharacterSet: storage.MySQLCharacterSet{Name: "utf8mb4", Collation: "utf8mb4_unicode_ci"}, SSHKeys: []storage.SSHKey{""}, AllowedCIDRs: []storage.IPv4CIDR{}},
		},
		{
			name:   "longQueryTime",
			create: mySQLCmd{LongQueryTime: storage.LongQueryTime("300"), SSHKeys: []storage.SSHKey{""}, AllowedCidrs: []storage.IPv4CIDR{}},
			want:   storage.MySQLParameters{LongQueryTime: storage.LongQueryTime("300"), SSHKeys: []storage.SSHKey{""}, AllowedCIDRs: []storage.IPv4CIDR{}},
		},
		{
			name:   "minWordLength",
			create: mySQLCmd{MinWordLength: ptr.To(5), SSHKeys: []storage.SSHKey{""}, AllowedCidrs: []storage.IPv4CIDR{}},
			want:   storage.MySQLParameters{MinWordLength: ptr.To(5), SSHKeys: []storage.SSHKey{""}, AllowedCIDRs: []storage.IPv4CIDR{}},
		},
		{
			name:   "transactionIsolation",
			create: mySQLCmd{TransactionIsolation: storage.MySQLTransactionCharacteristic("READ-UNCOMMITTED"), SSHKeys: []storage.SSHKey{""}, AllowedCidrs: []storage.IPv4CIDR{}},
			want:   storage.MySQLParameters{TransactionIsolation: storage.MySQLTransactionCharacteristic("READ-UNCOMMITTED"), SSHKeys: []storage.SSHKey{""}, AllowedCIDRs: []storage.IPv4CIDR{}},
		},
		{
			name:   "keepDailyBackups",
			create: mySQLCmd{KeepDailyBackups: ptr.To(5), SSHKeys: []storage.SSHKey{""}, AllowedCidrs: []storage.IPv4CIDR{}},
			want:   storage.MySQLParameters{KeepDailyBackups: ptr.To(5), SSHKeys: []storage.SSHKey{""}, AllowedCIDRs: []storage.IPv4CIDR{}},
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
			builder := fake.NewClientBuilder().WithScheme(scheme)
			if tt.interceptorFuncs != nil {
				builder = builder.WithInterceptorFuncs(*tt.interceptorFuncs)
			}
			client := builder.Build()
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
				t.Fatalf("expected mysql.Spec.ForProvider = %+v, got: %+v", created.Spec.ForProvider, tt.want)
			}
		})
	}
}
