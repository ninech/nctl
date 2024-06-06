package create

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestPostgres(t *testing.T) {
	tests := []struct {
		name             string
		create           postgresCmd
		want             storage.PostgresParameters
		wantErr          bool
		interceptorFuncs *interceptor.Funcs
	}{
		{
			name:   "simple",
			create: postgresCmd{},
			want:   storage.PostgresParameters{},
		},
		{
			name:    "simpleErrorOnCreation",
			create:  postgresCmd{},
			wantErr: true,
			interceptorFuncs: &interceptor.Funcs{
				Create: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.CreateOption) error {
					return errors.New("error on creation")
				},
			},
		},
		{
			name:   "machineType",
			create: postgresCmd{MachineType: infra.MachineType("nine-standard-1")},
			want:   storage.PostgresParameters{MachineType: infra.MachineType("nine-standard-1")},
		},
		{
			name:   "sshKeys",
			create: postgresCmd{SSHKeys: []storage.SSHKey{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJGG5/nnivrW4zLD4ANLclVT3y68GAg6NOA3HpzFLo5e test@test"}},
			want:   storage.PostgresParameters{SSHKeys: []storage.SSHKey{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJGG5/nnivrW4zLD4ANLclVT3y68GAg6NOA3HpzFLo5e test@test"}},
		},
		{
			name:   "allowedCIDRs",
			create: postgresCmd{AllowedCidrs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
			want:   storage.PostgresParameters{AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
		},
		{
			name:   "version",
			create: postgresCmd{PostgresVersion: storage.PostgresVersionDefault},
			want:   storage.PostgresParameters{Version: storage.PostgresVersionDefault},
		},
		{
			name:   "keepDailyBackups",
			create: postgresCmd{KeepDailyBackups: ptr.To(5)},
			want:   storage.PostgresParameters{KeepDailyBackups: ptr.To(5)},
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
			postgresClient := builder.Build()
			apiClient := &api.Client{WithWatch: postgresClient, Project: "default"}
			ctx := context.Background()

			if err := tt.create.Run(ctx, apiClient); (err != nil) != tt.wantErr {
				t.Errorf("postgresCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			created := &storage.Postgres{ObjectMeta: metav1.ObjectMeta{Name: tt.create.Name, Namespace: apiClient.Project}}
			if err := apiClient.Get(ctx, api.ObjectName(created), created); (err != nil) != tt.wantErr {
				t.Fatalf("expected postgres to exist, got: %s", err)
			}
			if tt.wantErr {
				return
			}

			// we set defaults for the slices
			if tt.want.AllowedCIDRs == nil {
				tt.want.AllowedCIDRs = []meta.IPv4CIDR{}
			}
			if tt.want.SSHKeys == nil {
				tt.want.SSHKeys = []storage.SSHKey{}
			}

			if !reflect.DeepEqual(created.Spec.ForProvider, tt.want) {
				t.Fatalf("expected postgres.Spec.ForProvider = %+v, got: %+v", created.Spec.ForProvider, tt.want)
			}
		})
	}
}
