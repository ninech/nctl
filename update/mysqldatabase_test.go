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

func TestMySQLDatabase(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		create  storage.MySQLDatabaseParameters
		update  mysqlDatabaseCmd
		want    storage.MySQLDatabaseParameters
		wantErr bool
	}{
		{
			name: "simple",
		},
		{
			name:    "empty-update",
			update:  mysqlDatabaseCmd{},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.update.Name = "test-" + t.Name()

			apiClient, err := test.SetupClient()
			require.NoError(t, err)

			created := test.MySQLDatabase(tt.update.Name, apiClient.Project, "nine-es34")
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(ctx, created); err != nil {
				t.Fatalf("mysqldatabase create error, got: %s", err)
			}
			if err := apiClient.Get(ctx, api.ObjectName(created), created); err != nil {
				t.Fatalf("expected mysqldatabase to exist, got: %s", err)
			}

			updated := &storage.MySQLDatabase{}
			if err := tt.update.Run(ctx, apiClient); (err != nil) != tt.wantErr {
				t.Errorf("mysqlDatabaseCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := apiClient.Get(ctx, api.ObjectName(created), updated); err != nil {
				t.Fatalf("expected mysqldatabase to exist, got: %s", err)
			}

			if !cmp.Equal(updated.Spec.ForProvider, tt.want) {
				t.Fatalf("expected mysqlDatabase.Spec.ForProvider = %v, got: %v", updated.Spec.ForProvider, tt.want)
			}
		})
	}
}
