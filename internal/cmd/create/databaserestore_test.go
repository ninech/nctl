package create

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/database"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/testutil"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestDatabaseRestore(t *testing.T) {
	t.Parallel()

	// the fake client mutates its objects, so every case builds its own
	backup := func(state storage.DatabaseJobState, version string) runtimeclient.Object {
		b := testutil.DatabaseBackup("mybackup", testutil.DefaultProject, database.Ref(storage.PostgresDatabaseKind, "source"))
		b.Status.AtProvider.State = state
		b.Status.AtProvider.Version.Database = version
		if state == storage.DatabaseJobStateSucceeded {
			b.Status.AtProvider.Path = "dumps/mybackup.dump"
		}
		return b
	}
	succeeded := func() runtimeclient.Object { return backup(storage.DatabaseJobStateSucceeded, "17.6") }
	postgres := func(version storage.PostgresVersion) *storage.PostgresDatabase {
		db := testutil.PostgresDatabase("mydb", testutil.DefaultProject, "nine-es34")
		db.Spec.ForProvider.Version = version
		return db
	}
	restore := func(name string, state storage.DatabaseJobState) *storage.DatabaseRestore {
		r := testutil.DatabaseRestore(name, testutil.DefaultProject, "mybackup", database.Ref(storage.PostgresDatabaseKind, "mydb"))
		r.Status.AtProvider.State = state
		return r
	}
	aged := func(name string, state storage.DatabaseJobState, age time.Duration) *storage.DatabaseRestore {
		r := restore(name, state)
		r.CreationTimestamp = metav1.NewTime(time.Now().Add(-age))
		return r
	}

	tests := []struct {
		name    string
		target  string
		objects []runtimeclient.Object
		force   bool
		// input answers the confirmation
		input       string
		wantErr     string
		wantCreated bool
		wantOut     string
		wantNotOut  string
		// admissionErr rejects the restore like the admission webhook
		admissionErr string
		// interrupt waits for the restore and interrupts the wait right away
		interrupt bool
		// startedDuringPrompt starts another restore while the prompt is open
		startedDuringPrompt bool
	}{
		{
			name: "postgres", target: "postgresdatabase/mydb", force: true, wantCreated: true,
			objects:    []runtimeclient.Object{succeeded(), postgres(storage.PostgresVersion17)},
			wantNotOut: "all data",
		},
		{
			name: "mysql", target: "mysqldatabase/mydb", force: true, wantCreated: true,
			objects: []runtimeclient.Object{succeeded(), testutil.MySQLDatabase("mydb", testutil.DefaultProject, "nine-es34")},
		},
		{
			name: "confirmed", target: "postgresdatabase/mydb", input: "y\n", wantCreated: true,
			objects: []runtimeclient.Object{succeeded(), postgres(storage.PostgresVersion17)},
			wantOut: `all data in postgresdatabase/mydb is replaced with the data of backup "mybackup"`,
		},
		{
			name: "declined", target: "postgresdatabase/mydb", input: "n\n",
			objects: []runtimeclient.Object{succeeded(), postgres(storage.PostgresVersion17)},
			wantOut: "DatabaseRestore canceled",
		},
		{
			name: "interrupted", target: "postgresdatabase/mydb", force: true, interrupt: true, wantCreated: true,
			objects: []runtimeclient.Object{succeeded(), postgres(storage.PostgresVersion17)},
			wantOut: "the restore continues on the server, check it with: nctl get databaserestores test-interrupted",
		},
		{
			name: "finishedRestore", target: "postgresdatabase/mydb", force: true, wantCreated: true,
			objects: []runtimeclient.Object{
				succeeded(), postgres(storage.PostgresVersion17),
				restore("done", storage.DatabaseJobStateSucceeded), restore("broken", storage.DatabaseJobStateFailed),
			},
		},
		{
			name: "activeRestore", target: "postgresdatabase/mydb", force: true,
			objects: []runtimeclient.Object{succeeded(), postgres(storage.PostgresVersion17), restore("busy", storage.DatabaseJobStateRunning)},
			wantErr: `restore "busy" into postgresdatabase/mydb is still in progress`,
		},
		{
			name: "pendingRestore", target: "postgresdatabase/mydb", force: true,
			objects: []runtimeclient.Object{succeeded(), postgres(storage.PostgresVersion17), restore("queued", storage.DatabaseJobStatePending)},
			wantErr: `restore "queued" into postgresdatabase/mydb is still in progress`,
		},
		{
			// a restore stuck in another state must not block new restores
			name: "unknownRestore", target: "postgresdatabase/mydb", force: true, wantCreated: true,
			objects: []runtimeclient.Object{succeeded(), postgres(storage.PostgresVersion17), aged("stuck", storage.DatabaseJobStateUnknown, time.Hour)},
		},
		{
			name: "startingRestore", target: "postgresdatabase/mydb", force: true,
			objects: []runtimeclient.Object{succeeded(), postgres(storage.PostgresVersion17), aged("starting", storage.DatabaseJobStateUnknown, time.Second)},
			wantErr: `restore "starting" into postgresdatabase/mydb is still in progress`,
		},
		{
			name: "startedDuringPrompt", target: "postgresdatabase/mydb", input: "y\n", startedDuringPrompt: true,
			objects: []runtimeclient.Object{succeeded(), postgres(storage.PostgresVersion17)},
			wantErr: `restore "other" into postgresdatabase/mydb is still in progress`,
		},
		{
			name: "noConfirmation", target: "postgresdatabase/mydb",
			objects: []runtimeclient.Object{succeeded(), postgres(storage.PostgresVersion17)},
			wantErr: "no confirmation received, pass --force to restore without asking",
		},
		{
			name: "backupFailed", target: "postgresdatabase/mydb", force: true,
			objects: []runtimeclient.Object{backup(storage.DatabaseJobStateFailed, ""), postgres(storage.PostgresVersion17)},
			wantErr: `backup "mybackup" is failed, only succeeded backups can be restored`,
		},
		{
			name: "backupPending", target: "postgresdatabase/mydb", force: true,
			objects: []runtimeclient.Object{backup(storage.DatabaseJobStatePending, ""), postgres(storage.PostgresVersion17)},
			wantErr: `backup "mybackup" is pending`,
		},
		{
			name: "targetDeleting", target: "postgresdatabase/mydb", force: true,
			objects: []runtimeclient.Object{succeeded(), func() runtimeclient.Object {
				db := postgres(storage.PostgresVersion17)
				db.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				db.Finalizers = []string{"test"}
				return db
			}()},
			wantErr: "target postgresdatabase/mydb is being deleted",
		},
		{
			name: "rejectedByAdmission", target: "postgresdatabase/mydb", input: "y\n",
			objects:      []runtimeclient.Object{succeeded(), postgres(storage.PostgresVersion17)},
			admissionErr: "cannot restore the backup into a database of version 17",
			wantErr:      "cannot restore the backup into a database of version 17",
			wantNotOut:   "all data",
		},
		{name: "unsupportedTarget", target: "keyvaluestore/mydb", force: true, wantErr: `unsupported database kind "keyvaluestore"`},
		{
			name: "targetMissing", target: "postgresdatabase/mydb", force: true,
			objects: []runtimeclient.Object{succeeded()},
			wantErr: "target postgresdatabase/mydb does not exist",
		},
		{
			name: "backupMissing", target: "postgresdatabase/mydb", force: true,
			objects: []runtimeclient.Object{postgres(storage.PostgresVersion17)},
			wantErr: `backup "mybackup" does not exist`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			out := &bytes.Buffer{}
			cmd := databaseRestoreCmd{Backup: "mybackup", Target: testutil.TypedReference(t, tt.target), Force: tt.force}
			is.NoError(cmd.BeforeApply(format.IOStreams{In: strings.NewReader(tt.input), Out: out, ErrOut: out}))
			cmd.Name = "test-" + tt.name
			cmd.Wait = false
			cmd.WaitTimeout = time.Second

			opts := []testutil.ClientSetupOption{testutil.WithObjects(tt.objects...)}
			funcs := interceptor.Funcs{}
			if tt.admissionErr != "" {
				funcs.Create = func(ctx context.Context, c runtimeclient.WithWatch, obj runtimeclient.Object, opts ...runtimeclient.CreateOption) error {
					if _, ok := obj.(*storage.DatabaseRestore); ok {
						return apierrors.NewForbidden(storage.SchemeGroupVersion.WithResource("databaserestores").GroupResource(), obj.GetName(), errors.New(tt.admissionErr))
					}
					return c.Create(ctx, obj, opts...)
				}
			}
			if tt.startedDuringPrompt {
				// the restore shows up from the second list, after the prompt
				lists := 0
				funcs.List = func(ctx context.Context, c runtimeclient.WithWatch, list runtimeclient.ObjectList, opts ...runtimeclient.ListOption) error {
					if err := c.List(ctx, list, opts...); err != nil {
						return err
					}
					if restores, ok := list.(*storage.DatabaseRestoreList); ok {
						if lists++; lists > 1 {
							restores.Items = append(restores.Items, *restore("other", storage.DatabaseJobStateRunning))
						}
					}
					return nil
				}
			}
			opts = append(opts, testutil.WithInterceptorFuncs(funcs))
			apiClient := testutil.SetupClient(t, opts...)

			ctx := t.Context()
			if tt.interrupt {
				cmd.Wait = true
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			err := cmd.Run(ctx, apiClient)
			if tt.wantErr != "" {
				is.ErrorContains(err, tt.wantErr)
			} else {
				is.NoError(err)
			}
			if tt.wantOut != "" {
				is.Contains(out.String(), tt.wantOut)
			}
			if tt.wantNotOut != "" {
				is.NotContains(out.String(), tt.wantNotOut)
			}

			created := &storage.DatabaseRestore{ObjectMeta: metav1.ObjectMeta{Name: cmd.Name, Namespace: apiClient.Project}}
			err = apiClient.Get(t.Context(), api.ObjectName(created), created)
			if !tt.wantCreated {
				is.True(apierrors.IsNotFound(err), "restore must not be created, got %v", err)
				return
			}
			is.NoError(err)
			is.Equal("mybackup", created.Spec.ForProvider.Backup.Name)
			want, err := database.LocalRef(cmd.Target.TypedReference)
			is.NoError(err)
			is.Equal(want, created.Spec.ForProvider.Target)
			is.Equal("jrocket@example.com", created.Annotations[api.CreatedByAnnotation])
		})
	}
}
