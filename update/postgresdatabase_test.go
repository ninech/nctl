package update

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
)

func TestPostgresDatabase(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

			out := &bytes.Buffer{}
			tt.update.Writer = format.NewWriter(out)
			tt.update.Name = "test-" + t.Name()

			apiClient, err := test.SetupClient()
			if err != nil {
				t.Fatalf("setup client error, got: %s", err)
			}

			created := test.PostgresDatabase(tt.update.Name, apiClient.Project, "nine-es34")
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(t.Context(), created); err != nil {
				t.Fatalf("postgresdatabase create error, got: %s", err)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), created); err != nil {
				t.Fatalf("expected postgresdatabase to exist, got: %s", err)
			}

			updated := &storage.PostgresDatabase{}
			if err := tt.update.Run(t.Context(), apiClient); (err != nil) != tt.wantErr {
				t.Errorf("postgresDatabaseCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), updated); err != nil {
				t.Fatalf("expected postgresdatabase to exist, got: %s", err)
			}

			if !cmp.Equal(updated.Spec.ForProvider, tt.want) {
				t.Fatalf("expected postgresDatabase.Spec.ForProvider = %v, got: %v", updated.Spec.ForProvider, tt.want)
			}

			if !tt.wantErr {
				if !strings.Contains(out.String(), "updated") {
					t.Fatalf("expected output to contain 'updated', got: %s", out.String())
				}
				if !strings.Contains(out.String(), tt.update.Name) {
					t.Fatalf("expected output to contain %s, got: %s", tt.update.Name, out.String())
				}
			}
		})
	}
}
