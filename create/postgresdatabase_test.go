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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestPostgresDatabase(t *testing.T) {
	t.Parallel()

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
		{
			name:   "collation",
			create: postgresDatabaseCmd{Collation: storage.PostgresDatabaseCollationDefault},
			want:   storage.PostgresDatabaseParameters{Collation: storage.PostgresDatabaseCollationDefault},
		},
		{
			name:   "restoreFrom",
			create: postgresDatabaseCmd{RestoreFrom: "mybackup"},
			want:   storage.PostgresDatabaseParameters{RestoreFrom: &meta.LocalReference{Name: "mybackup"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			tt.create.Name = "test-" + t.Name()
			tt.create.Wait = false
			tt.create.WaitTimeout = time.Second

			opts := []test.ClientSetupOption{}
			if tt.interceptorFuncs != nil {
				opts = append(opts, test.WithInterceptorFuncs(*tt.interceptorFuncs))
			}
			apiClient := test.SetupClient(t, opts...)

			if err := tt.create.Run(t.Context(), apiClient); (err != nil) != tt.wantErr {
				t.Errorf("postgresDatabaseCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			created := &storage.PostgresDatabase{ObjectMeta: metav1.ObjectMeta{Name: tt.create.Name, Namespace: apiClient.Project}}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), created); (err != nil) != tt.wantErr {
				t.Fatalf("expected postgresdatabase to exist, got: %s", err)
			}
			if tt.wantErr {
				return
			}

			is.True(cmp.Equal(tt.want, created.Spec.ForProvider))
		})
	}
}

// TestPostgresDatabaseAlreadyExists covers creating a database whose name is
// already taken.
func TestPostgresDatabaseAlreadyExists(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	existing := test.PostgresDatabase("dup", test.DefaultProject, "nine-es34")
	apiClient := test.SetupClient(t, test.WithObjects(existing))

	cmd := postgresDatabaseCmd{}
	cmd.Name = "dup"
	cmd.Wait = false
	cmd.WaitTimeout = time.Second

	err := cmd.Run(t.Context(), apiClient)
	is.ErrorContains(err, `PostgresDatabase "dup" already exists in project`)
}
