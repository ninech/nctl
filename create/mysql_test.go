package create

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestMySQL(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name             string
		create           mySQLCmd
		want             storage.MySQLParameters
		wantErr          bool
		interceptorFuncs *interceptor.Funcs
	}{
		{
			name:   "simple",
			create: mySQLCmd{},
			want:   storage.MySQLParameters{},
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
			create: mySQLCmd{MachineType: storage.MySQLMachineTypeDefault.String()},
			want:   storage.MySQLParameters{MachineType: storage.MySQLMachineTypeDefault},
		},
		{
			name:   "sshKeys",
			create: mySQLCmd{SSHKeys: []storage.SSHKey{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJGG5/nnivrW4zLD4ANLclVT3y68GAg6NOA3HpzFLo5e test@test"}},
			want:   storage.MySQLParameters{SSHKeys: []storage.SSHKey{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJGG5/nnivrW4zLD4ANLclVT3y68GAg6NOA3HpzFLo5e test@test"}},
		},
		{
			name:   "sqlMode",
			create: mySQLCmd{SQLMode: &[]storage.MySQLMode{"ONLY_FULL_GROUP_BY"}},
			want:   storage.MySQLParameters{SQLMode: &[]storage.MySQLMode{"ONLY_FULL_GROUP_BY"}},
		},
		{
			name:   "allowedCIDRs",
			create: mySQLCmd{AllowedCidrs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
			want:   storage.MySQLParameters{AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
		},
		{
			name:   "characterSet",
			create: mySQLCmd{CharacterSetName: "utf8mb4", CharacterSetCollation: "utf8mb4_unicode_ci"},
			want:   storage.MySQLParameters{CharacterSet: storage.MySQLCharacterSet{Name: "utf8mb4", Collation: "utf8mb4_unicode_ci"}},
		},
		{
			name:   "longQueryTime",
			create: mySQLCmd{LongQueryTime: storage.LongQueryTime("300")},
			want:   storage.MySQLParameters{LongQueryTime: storage.LongQueryTime("300")},
		},
		{
			name:   "minWordLength",
			create: mySQLCmd{MinWordLength: ptr.To(5)},
			want:   storage.MySQLParameters{MinWordLength: ptr.To(5)},
		},
		{
			name:   "transactionIsolation",
			create: mySQLCmd{TransactionIsolation: storage.MySQLTransactionCharacteristic("READ-UNCOMMITTED")},
			want:   storage.MySQLParameters{TransactionIsolation: storage.MySQLTransactionCharacteristic("READ-UNCOMMITTED")},
		},
		{
			name:   "keepDailyBackups",
			create: mySQLCmd{KeepDailyBackups: ptr.To(5)},
			want:   storage.MySQLParameters{KeepDailyBackups: ptr.To(5)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.create.Name = "test-" + t.Name()
			tt.create.Wait = false
			tt.create.WaitTimeout = time.Second

			opts := []test.ClientSetupOption{}
			if tt.interceptorFuncs != nil {
				opts = append(opts, test.WithInterceptorFuncs(*tt.interceptorFuncs))
			}
			apiClient, err := test.SetupClient(opts...)
			require.NoError(t, err)

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

			require.True(t, cmp.Equal(tt.want, created.Spec.ForProvider))
		})
	}
}
