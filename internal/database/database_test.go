package database

import (
	"strings"
	"testing"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/internal/testutil"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func TestLocalRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ref      string
		wantErr  string
		wantKind string
	}{
		{ref: "postgresdatabase/mydb", wantKind: storage.PostgresDatabaseKind},
		{ref: "PostgresDatabase/mydb", wantKind: storage.PostgresDatabaseKind},
		{ref: "mysqldatabase/mydb", wantKind: storage.MySQLDatabaseKind},
		{ref: "keyvaluestore/mydb", wantErr: `unsupported database kind "keyvaluestore"`},
		{ref: "postgres/mydb", wantErr: `unsupported database kind "postgres"`},
	}
	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			got, err := LocalRef(testutil.TypedReference(t, tt.ref).TypedReference)
			if tt.wantErr != "" {
				is.ErrorContains(err, tt.wantErr)
				return
			}
			is.NoError(err)
			is.Equal(Ref(tt.wantKind, "mydb"), got)
			is.Equal(strings.ToLower(tt.ref), FormatRef(got.Kind, got.Name))
		})
	}
}

func TestSameRef(t *testing.T) {
	ref := func(group, kind, name string) meta.LocalTypedReference {
		return meta.LocalTypedReference{
			LocalReference: meta.LocalReference{Name: name},
			GroupKind:      metav1.GroupKind{Group: group, Kind: kind},
		}
	}
	pg := Ref(storage.PostgresDatabaseKind, "mydb")

	tests := map[string]struct {
		a, b meta.LocalTypedReference
		want bool
	}{
		"identical":             {pg, pg, true},
		"group ignored":         {pg, ref("", storage.PostgresDatabaseKind, "mydb"), true},
		"kind case insensitive": {pg, ref(storage.Group, "postgresdatabase", "mydb"), true},
		"different name":        {pg, Ref(storage.PostgresDatabaseKind, "other"), false},
		"different kind":        {pg, Ref(storage.MySQLDatabaseKind, "mydb"), false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tt.want, SameRef(tt.a, tt.b))
		})
	}
}

func TestNewRestore(t *testing.T) {
	target := Ref(storage.PostgresDatabaseKind, "mydb")

	t.Run("empty name generates one from the target", func(t *testing.T) {
		is := require.New(t)
		r := NewRestore("", "proj", "mybackup", target)
		is.Empty(r.Name)
		is.Equal("postgresdatabase-mydb-", r.GenerateName)
		is.Equal("mybackup", r.Spec.ForProvider.Backup.Name)
		is.Equal(target, r.Spec.ForProvider.Target)
	})

	t.Run("explicit name is kept", func(t *testing.T) {
		is := require.New(t)
		r := NewRestore("chosen", "proj", "mybackup", target)
		is.Equal("chosen", r.Name)
		is.Empty(r.GenerateName)
	})
}

func TestRestoreDone(t *testing.T) {
	t.Parallel()

	restore := func(state storage.DatabaseJobState) *storage.DatabaseRestore {
		r := NewRestore("r1", "project", "b1", Ref(storage.PostgresDatabaseKind, "mydb"))
		r.Status.AtProvider.State = state
		return r
	}

	tests := []struct {
		name     string
		event    watch.Event
		wantDone bool
		wantErr  string
	}{
		{name: "pending", event: watch.Event{Type: watch.Modified, Object: restore(storage.DatabaseJobStatePending)}},
		{name: "succeeded", event: watch.Event{Type: watch.Modified, Object: restore(storage.DatabaseJobStateSucceeded)}, wantDone: true},
		{name: "failed", event: watch.Event{Type: watch.Modified, Object: restore(storage.DatabaseJobStateFailed)}, wantErr: "restore failed"},
		{
			name:    "deleted",
			event:   watch.Event{Type: watch.Deleted, Object: restore(storage.DatabaseJobStateRunning)},
			wantErr: `the restore "r1" into postgresdatabase/mydb was deleted before it finished`,
		},
		{
			// the final state counts, e.g. when a reconnected watch only sees the deletion
			name:     "deletedAfterSuccess",
			event:    watch.Event{Type: watch.Deleted, Object: restore(storage.DatabaseJobStateSucceeded)},
			wantDone: true,
		},
		{name: "otherObject", event: watch.Event{Type: watch.Modified, Object: &storage.DatabaseBackup{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			done, err := RestoreDone(tt.event)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.wantDone, done)
		})
	}
}
