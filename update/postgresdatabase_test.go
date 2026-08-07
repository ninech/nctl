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
	"k8s.io/utils/ptr"
)

func TestPostgresDatabase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		create storage.PostgresDatabaseParameters
		update postgresDatabaseCmd
		want   storage.PostgresDatabaseParameters
		// wantUnchanged expects the update to be a no-op, which succeeds
		// without writing the database.
		wantUnchanged bool
	}{
		{
			name:          "simple",
			wantUnchanged: true,
		},
		{
			name:          "empty-update",
			update:        postgresDatabaseCmd{},
			wantUnchanged: true,
		},
		{
			name:   "update-backup-schedule",
			create: storage.PostgresDatabaseParameters{BackupSchedule: storage.DatabaseBackupScheduleCalendarDisabled},
			update: postgresDatabaseCmd{BackupSchedule: ptr.To(storage.DatabaseBackupScheduleCalendarDaily)},
			want:   storage.PostgresDatabaseParameters{BackupSchedule: storage.DatabaseBackupScheduleCalendarDaily},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := &bytes.Buffer{}
			tt.update.Writer = format.NewWriter(out)
			tt.update.Name = "test-" + t.Name()

			apiClient := test.SetupClient(t)

			created := test.PostgresDatabase(tt.update.Name, apiClient.Project, "nine-es34")
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(t.Context(), created); err != nil {
				t.Fatalf("postgresdatabase create error, got: %s", err)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), created); err != nil {
				t.Fatalf("expected postgresdatabase to exist, got: %s", err)
			}

			updated := &storage.PostgresDatabase{}
			if err := tt.update.Run(t.Context(), apiClient); err != nil {
				t.Errorf("postgresDatabaseCmd.Run() error = %v", err)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), updated); err != nil {
				t.Fatalf("expected postgresdatabase to exist, got: %s", err)
			}

			if !cmp.Equal(updated.Spec.ForProvider, tt.want) {
				t.Fatalf("expected postgresDatabase.Spec.ForProvider = %v, got: %v", updated.Spec.ForProvider, tt.want)
			}

			wantOutput := "updated"
			if tt.wantUnchanged {
				wantOutput = "no changes made"
			}
			if !strings.Contains(out.String(), wantOutput) {
				t.Fatalf("expected output to contain %q, got: %s", wantOutput, out.String())
			}
			if !strings.Contains(out.String(), tt.update.Name) {
				t.Fatalf("expected output to contain %s, got: %s", tt.update.Name, out.String())
			}
		})
	}
}
