package database

import (
	"strings"
	"testing"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

func TestParseRef(t *testing.T) {
	tests := []struct {
		ref      string
		wantErr  bool
		wantKind string
		wantName string
	}{
		{ref: "postgresdatabase/mydb", wantKind: storage.PostgresDatabaseKind, wantName: "mydb"},
		{ref: "PostgresDatabase/mydb", wantKind: storage.PostgresDatabaseKind, wantName: "mydb"},
		{ref: "mysqldatabase/other", wantKind: storage.MySQLDatabaseKind, wantName: "other"},
		{ref: "mydb", wantErr: true},
		{ref: "postgresdatabase/", wantErr: true},
		{ref: "/mydb", wantErr: true},
		{ref: "keyvaluestore/mydb", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			is := require.New(t)
			got, err := ParseRef(tt.ref)
			if tt.wantErr {
				is.Error(err)
				return
			}
			is.NoError(err)
			is.Equal(tt.wantKind, got.Kind)
			is.Equal(tt.wantName, got.Name)
			is.Equal(storage.Group, got.Group)
			// parse and format round-trip
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

// TestBootstrapCompleted covers the outcomes a database records for its
// bootstrap restore. The recorded shape must be the one the controller
// writes.
func TestBootstrapCompleted(t *testing.T) {
	done := BootstrapCompleted(storage.PostgresDatabaseKind, "mydb")

	newDB := func(name string, bootstrap *storage.BootstrapStatus) *storage.PostgresDatabase {
		db := &storage.PostgresDatabase{ObjectMeta: metav1.ObjectMeta{Name: name}}
		db.Status.AtProvider.Bootstrap = bootstrap
		return db
	}
	recorded := func(state storage.DatabaseRestoreState) *storage.BootstrapStatus {
		return &storage.BootstrapStatus{State: state, Restore: "postgresdatabase-mydb", End: metav1.Now()}
	}

	tests := map[string]struct {
		db       *storage.PostgresDatabase
		finished bool
		wantErr  string
	}{
		"not recorded yet": {db: newDB("mydb", nil)},
		"succeeded":        {db: newDB("mydb", recorded(storage.DatabaseRestoreStateSucceeded)), finished: true},
		"failed":           {db: newDB("mydb", recorded(storage.DatabaseRestoreStateFailed)), wantErr: "postgresdatabase-mydb"},
		"another database": {db: newDB("otherdb", recorded(storage.DatabaseRestoreStateSucceeded))},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			is := require.New(t)
			finished, err := done(watch.Event{Type: watch.Modified, Object: tt.db})
			is.Equal(tt.finished, finished)
			if tt.wantErr == "" {
				is.NoError(err)
				return
			}
			is.ErrorIs(err, ErrRestoreFailed)
			is.ErrorContains(err, tt.wantErr, "the failure should name the restore to inspect")
		})
	}
}
