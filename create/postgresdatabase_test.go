package create

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestPostgresDatabase(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name             string
		create           postgresDatabaseCmd
		want             storage.PostgresDatabaseParameters
		wantErr          bool
		interceptorFuncs *interceptor.Funcs
	}{
		{
			name:   "simple",
			create: postgresDatabaseCmd{},
			want:   storage.PostgresDatabaseParameters{},
		},
		{
			name:    "simpleErrorOnCreation",
			create:  postgresDatabaseCmd{},
			wantErr: true,
			interceptorFuncs: &interceptor.Funcs{
				Create: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.CreateOption) error {
					return errors.New("error on creation")
				},
			},
		},
		{
			name:   "version",
			create: postgresDatabaseCmd{PostgresDatabaseVersion: storage.PostgresDatabaseVersionDefault},
			want:   storage.PostgresDatabaseParameters{Version: storage.PostgresDatabaseVersionDefault},
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
				t.Errorf("postgresDatabaseCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			created := &storage.PostgresDatabase{ObjectMeta: metav1.ObjectMeta{Name: tt.create.Name, Namespace: apiClient.Project}}
			if err := apiClient.Get(ctx, api.ObjectName(created), created); (err != nil) != tt.wantErr {
				t.Fatalf("expected postgresdatabase to exist, got: %s", err)
			}
			if tt.wantErr {
				return
			}

			require.True(t, cmp.Equal(tt.want, created.Spec.ForProvider))
		})
	}
}
