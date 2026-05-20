package update

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestMySQLDatabase(t *testing.T) {
	t.Parallel()

	noFlagsInterceptor := &interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			oldRV := obj.GetResourceVersion()
			if err := c.Update(ctx, obj, opts...); err != nil {
				return err
			}
			obj.SetResourceVersion(oldRV)
			return nil
		},
	}

	tests := []struct {
		name             string
		create           storage.MySQLDatabaseParameters
		update           mysqlDatabaseCmd
		want             storage.MySQLDatabaseParameters
		wantErr          bool
		interceptorFuncs *interceptor.Funcs
	}{
		{
			name:             "no-flags",
			wantErr:          true,
			interceptorFuncs: noFlagsInterceptor,
		},
		{
			name: "simple",
		},
		{
			name:    "empty-update",
			update:  mysqlDatabaseCmd{},
			wantErr: false,
		},
		{
			name:   "update-backup-schedule",
			create: storage.MySQLDatabaseParameters{BackupSchedule: storage.DatabaseBackupScheduleCalendarDaily},
			update: mysqlDatabaseCmd{BackupSchedule: ptr.To(storage.DatabaseBackupScheduleCalendarDisabled)},
			want:   storage.MySQLDatabaseParameters{BackupSchedule: storage.DatabaseBackupScheduleCalendarDisabled},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := &bytes.Buffer{}
			tt.update.Writer = format.NewWriter(out)
			tt.update.Name = "test-" + t.Name()

			var opts []test.ClientSetupOption
			if tt.interceptorFuncs != nil {
				opts = append(opts, test.WithInterceptorFuncs(*tt.interceptorFuncs))
			}
			apiClient := test.SetupClient(t, opts...)

			created := test.MySQLDatabase(tt.update.Name, apiClient.Project, "nine-es34")
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(t.Context(), created); err != nil {
				t.Fatalf("mysqldatabase create error, got: %s", err)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), created); err != nil {
				t.Fatalf("expected mysqldatabase to exist, got: %s", err)
			}

			updated := &storage.MySQLDatabase{}
			if err := tt.update.Run(t.Context(), apiClient); (err != nil) != tt.wantErr {
				t.Errorf("mysqlDatabaseCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), updated); err != nil {
				t.Fatalf("expected mysqldatabase to exist, got: %s", err)
			}

			if !cmp.Equal(updated.Spec.ForProvider, tt.want) {
				t.Fatalf("expected mysqlDatabase.Spec.ForProvider = %v, got: %v", updated.Spec.ForProvider, tt.want)
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
