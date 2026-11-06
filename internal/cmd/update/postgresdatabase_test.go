package update

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			update: postgresDatabaseCmd{BackupSchedule: new(storage.DatabaseBackupScheduleCalendarDaily)},
			want:   storage.PostgresDatabaseParameters{BackupSchedule: storage.DatabaseBackupScheduleCalendarDaily},
		},
		{
			name:   "update-backup-daily-time",
			create: storage.PostgresDatabaseParameters{BackupDailyTime: "03:00"},
			update: postgresDatabaseCmd{BackupDailyTime: new("22:45")},
			want:   storage.PostgresDatabaseParameters{BackupDailyTime: "22:45"},
		},
		{
			name:   "update-backup-ttl",
			create: storage.PostgresDatabaseParameters{BackupTTL: &metav1.Duration{Duration: 240 * time.Hour}},
			update: postgresDatabaseCmd{BackupTTL: new(72 * time.Hour)},
			want:   storage.PostgresDatabaseParameters{BackupTTL: &metav1.Duration{Duration: 72 * time.Hour}},
		},
		{
			name:   "reset-backup-ttl",
			create: storage.PostgresDatabaseParameters{BackupTTL: &metav1.Duration{Duration: 72 * time.Hour}},
			update: postgresDatabaseCmd{BackupTTL: new(time.Duration(0))},
			want:   storage.PostgresDatabaseParameters{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := &bytes.Buffer{}
			tt.update.Writer = format.NewWriter(out, out)
			tt.update.Name = "test-" + t.Name()

			apiClient := testutil.SetupClient(t)

			created := testutil.PostgresDatabase(tt.update.Name, apiClient.Project, "nine-es34")
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
