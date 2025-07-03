package update

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
)

func TestPostgresDatabase(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		create  storage.PostgresDatabaseParameters
		update  postgresDatabaseCmd
		want    storage.PostgresDatabaseParameters
		wantErr bool
	}{
		{
			name: "simple",
		},
		{
			name:    "empty-update",
			update:  postgresDatabaseCmd{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.update.Name = "test-" + t.Name()

			apiClient, err := test.SetupClient()
			require.NoError(t, err)

			created := test.PostgresDatabase(tt.update.Name, apiClient.Project, "nine-es34")
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(ctx, created); err != nil {
				t.Fatalf("postgresdatabase create error, got: %s", err)
			}
			if err := apiClient.Get(ctx, api.ObjectName(created), created); err != nil {
				t.Fatalf("expected postgresdatabase to exist, got: %s", err)
			}

			updated := &storage.PostgresDatabase{}
			if err := tt.update.Run(ctx, apiClient); (err != nil) != tt.wantErr {
				t.Errorf("postgresDatabaseCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := apiClient.Get(ctx, api.ObjectName(created), updated); err != nil {
				t.Fatalf("expected postgresdatabase to exist, got: %s", err)
			}

			if !cmp.Equal(updated.Spec.ForProvider, tt.want) {
				t.Fatalf("expected postgresDatabase.Spec.ForProvider = %v, got: %v", updated.Spec.ForProvider, tt.want)
			}
		})
	}
}
