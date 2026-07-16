package copy

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/database"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

const sourceName, targetName = "source-db", "copy-db"

// TestCopyDatabase covers the default path. The new database is cloned from a
// fresh backup of the source via cloneFrom. No existing backup is required.
func TestCopyDatabase(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		kind    string
		objects func() []client.Object
		newCmd  func() runner
		verify  func(is *require.Assertions, client *api.Client)
	}{
		"postgres": {
			kind: storage.PostgresDatabaseKind,
			objects: func() []client.Object {
				source := test.PostgresDatabase(sourceName, test.DefaultProject, "nine-es34")
				source.Spec.ForProvider.InstanceReference = &meta.Reference{Name: "some-instance"}
				return []client.Object{source}
			},
			newCmd: func() runner { return &postgresDatabaseCmd{baseCmd()} },
			verify: func(is *require.Assertions, c *api.Client) {
				db := &storage.PostgresDatabase{}
				is.NoError(c.Get(t.Context(), c.Name(targetName), db))
				is.NotNil(db.Spec.ForProvider.CloneFrom)
				is.Equal(sourceName, db.Spec.ForProvider.CloneFrom.Name)
				is.Nil(db.Spec.ForProvider.RestoreFrom)
				is.Nil(db.Spec.ForProvider.InstanceReference)
				is.Equal("postgresdatabase-"+targetName, db.Spec.WriteConnectionSecretToReference.Name)
			},
		},
		"mysql": {
			kind: storage.MySQLDatabaseKind,
			objects: func() []client.Object {
				source := test.MySQLDatabase(sourceName, test.DefaultProject, "nine-es34")
				source.Spec.ForProvider.InstanceReference = &meta.Reference{Name: "some-instance"}
				return []client.Object{source}
			},
			newCmd: func() runner { return &mysqlDatabaseCmd{baseCmd()} },
			verify: func(is *require.Assertions, c *api.Client) {
				db := &storage.MySQLDatabase{}
				is.NoError(c.Get(t.Context(), c.Name(targetName), db))
				is.NotNil(db.Spec.ForProvider.CloneFrom)
				is.Equal(sourceName, db.Spec.ForProvider.CloneFrom.Name)
				is.Nil(db.Spec.ForProvider.RestoreFrom)
				is.Nil(db.Spec.ForProvider.InstanceReference)
				is.Equal("mysqldatabase-"+targetName, db.Spec.WriteConnectionSecretToReference.Name)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			apiClient := test.SetupClient(t, test.WithObjects(tc.objects()...))
			defer simulateRestore(t, apiClient, tc.kind)()

			is.NoError(tc.newCmd().Run(t.Context(), apiClient))
			tc.verify(is, apiClient)
		})
	}
}

// TestCopyDatabaseVersion covers the --target-version override and the default
// of inheriting the source version.
func TestCopyDatabaseVersion(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		targetVersion string
		want          storage.PostgresVersion
	}{
		"inherits source version": {targetVersion: "", want: storage.PostgresVersion17},
		"overrides version":       {targetVersion: "18", want: storage.PostgresVersion("18")},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			source := test.PostgresDatabase(sourceName, test.DefaultProject, "nine-es34")
			source.Spec.ForProvider.Version = storage.PostgresVersion17
			apiClient := test.SetupClient(t, test.WithObjects(source))
			defer simulateRestore(t, apiClient, storage.PostgresDatabaseKind)()

			cmd := baseCmd()
			cmd.TargetVersion = tc.targetVersion
			is.NoError((&postgresDatabaseCmd{cmd}).Run(t.Context(), apiClient))

			db := &storage.PostgresDatabase{}
			is.NoError(apiClient.Get(t.Context(), apiClient.Name(targetName), db))
			is.Equal(tc.want, db.Spec.ForProvider.Version)
		})
	}
}

// TestCopyDatabaseTargetExists covers a copy whose target name is already taken.
func TestCopyDatabaseTargetExists(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	source := test.PostgresDatabase(sourceName, test.DefaultProject, "nine-es34")
	existing := test.PostgresDatabase(targetName, test.DefaultProject, "nine-es34")
	apiClient := test.SetupClient(t, test.WithObjects(source, existing))

	cmd := baseCmd()
	cmd.WaitTimeout = time.Second
	err := (&postgresDatabaseCmd{cmd}).Run(t.Context(), apiClient)
	is.ErrorContains(err, `PostgresDatabase "copy-db" already exists in project`)
}

func TestCopyDatabaseInterrupted(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	apiClient, cmd := postgresCopy(t)

	ctx, cancel := context.WithCancel(t.Context())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	is.NoError(cmd.Run(ctx, apiClient))
	is.NotNil(cloneFromOf(t, apiClient))
}

func TestCopyDatabaseNoWait(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	apiClient, cmd := postgresCopy(t)
	cmd.Wait = false

	is.NoError(cmd.Run(t.Context(), apiClient))
	is.NotNil(cloneFromOf(t, apiClient))
}

// TestCopyDatabaseWatchDrop covers a wait whose restore watch drops, as
// happens when the API server recycles a long watch. The wait reconnects
// instead of failing.
func TestCopyDatabaseWatchDrop(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	var watches atomic.Int32
	source := test.PostgresDatabase(sourceName, test.DefaultProject, "nine-es34")
	apiClient := test.SetupClient(t,
		test.WithObjects(source),
		test.WithInterceptorFuncs(interceptor.Funcs{
			Watch: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) (watch.Interface, error) {
				if watches.Add(1) == 1 {
					// the first watch drops immediately, as a recycled API
					// server connection would
					dropped := watch.NewFake()
					dropped.Stop()
					return dropped, nil
				}
				return c.Watch(ctx, list, opts...)
			},
		}),
	)
	defer simulateRestore(t, apiClient, storage.PostgresDatabaseKind)()

	cmd := postgresDatabaseCmd{baseCmd()}
	is.NoError(cmd.Run(t.Context(), apiClient))
	is.GreaterOrEqual(watches.Load(), int32(2), "the wait should have reconnected after the dropped watch")
}

// TestCopyDatabaseRestoreFailed covers a copy whose composed restore fails:
// the command reports the failure instead of waiting for the timeout.
func TestCopyDatabaseRestoreFailed(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	apiClient, cmd := postgresCopy(t)
	defer simulateRestoreResult(t, apiClient, storage.PostgresDatabaseKind, storage.DatabaseRestoreStateFailed)()

	err := cmd.Run(t.Context(), apiClient)
	is.ErrorContains(err, "failed")
	is.ErrorContains(err, "nctl delete postgresdatabase "+targetName, "the resolution should be spelled out")
}

func baseCmd() databaseCmd {
	// Wait is set explicitly, as kong defaults do not apply to struct
	// literals.
	return databaseCmd{Name: sourceName, TargetName: targetName, Wait: true, WaitTimeout: 10 * time.Second}
}

func postgresCopy(t *testing.T) (*api.Client, *postgresDatabaseCmd) {
	source := test.PostgresDatabase(sourceName, test.DefaultProject, "nine-es34")
	apiClient := test.SetupClient(t, test.WithObjects(source))
	return apiClient, &postgresDatabaseCmd{baseCmd()}
}

func cloneFromOf(t *testing.T, apiClient *api.Client) *meta.LocalReference {
	db := &storage.PostgresDatabase{}
	require.NoError(t, apiClient.Get(t.Context(), apiClient.Name(targetName), db))
	return db.Spec.ForProvider.CloneFrom
}

// simulateRestore records a succeeded bootstrap restore on the copied
// database, so the copy completes.
func simulateRestore(t *testing.T, apiClient *api.Client, kind string) (stop func()) {
	return simulateRestoreResult(t, apiClient, kind, storage.DatabaseRestoreStateSucceeded)
}

// simulateRestoreResult records the given bootstrap restore state on the copied
// database, as the database controller does.
func simulateRestoreResult(t *testing.T, apiClient *api.Client, kind string, state storage.DatabaseRestoreState) (stop func()) {
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	db, err := database.New(kind)
	require.NoError(t, err)

	go func() {
		defer close(done)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if apiClient.Get(ctx, apiClient.Name(targetName), db) != nil {
					continue
				}
				db.(storage.BootstrapRecorder).SetBootstrap(storage.BootstrapStatus{
					State:   state,
					Restore: strings.ToLower(kind) + "-" + targetName,
					End:     metav1.Now(),
				})
				_ = apiClient.Update(ctx, db)
				return
			}
		}
	}()

	return func() { cancel(); <-done }
}

type runner interface {
	Run(ctx context.Context, client *api.Client) error
}
