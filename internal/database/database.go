// Package database contains helpers shared by the database resource commands
package database

import (
	"errors"
	"fmt"
	"strings"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// kinds are the database kinds which support backups and restores.
var (
	kinds     = []string{storage.PostgresDatabaseKind, storage.MySQLDatabaseKind}
	kindNames = strings.ToLower(strings.Join(kinds, ", "))
)

// Ref returns a reference to the named database of the given kind.
func Ref(kind, name string) meta.LocalTypedReference {
	return meta.LocalTypedReference{
		LocalReference: meta.LocalReference{Name: name},
		GroupKind:      metav1.GroupKind{Group: storage.Group, Kind: kind},
	}
}

// ParseRef parses a "kind/name" database reference (e.g.
// "postgresdatabase/mydb"). The kind is matched case-insensitively.
func ParseRef(ref string) (meta.LocalTypedReference, error) {
	kind, name, ok := strings.Cut(ref, "/")
	if ok && name != "" {
		for _, k := range kinds {
			if strings.EqualFold(kind, k) {
				return Ref(k, name), nil
			}
		}
	}
	return meta.LocalTypedReference{}, fmt.Errorf(
		"invalid database reference %q, expected <kind>/<name> where kind is one of: %s",
		ref, kindNames,
	)
}

// New returns an empty database object of the given kind, suitable as a target
// for a Get.
func New(kind string) (runtimeclient.Object, error) {
	switch kind {
	case storage.PostgresDatabaseKind:
		return &storage.PostgresDatabase{}, nil
	case storage.MySQLDatabaseKind:
		return &storage.MySQLDatabase{}, nil
	}
	return nil, fmt.Errorf("unsupported database kind %q, expected one of: %s", kind, kindNames)
}

// FormatRef formats a database reference as "<kind>/<name>".
func FormatRef(kind, name string) string {
	return strings.ToLower(kind) + "/" + name
}

// SameRef reports whether two references point at the same database, comparing
// kind (case-insensitively) and name while ignoring the group.
func SameRef(a, b meta.LocalTypedReference) bool {
	return strings.EqualFold(a.Kind, b.Kind) && a.Name == b.Name
}

// NewRestore returns a DatabaseRestore which restores the named backup into the
// given target database. When name is empty the restore is named after the
// target, matching the controller-composed restores of a copy, with a
// server-generated suffix. All other fields are defaulted server-side.
func NewRestore(name, project, backup string, target meta.LocalTypedReference) *storage.DatabaseRestore {
	objectMeta := metav1.ObjectMeta{Namespace: project}
	if name == "" {
		objectMeta.GenerateName = strings.ToLower(target.Kind) + "-" + target.Name + "-"
	} else {
		objectMeta.Name = name
	}
	return &storage.DatabaseRestore{
		ObjectMeta: objectMeta,
		Spec: storage.DatabaseRestoreSpec{
			ForProvider: storage.DatabaseRestoreParameters{
				Backup: meta.LocalReference{Name: backup},
				Target: target,
			},
		},
	}
}

// ListFor returns an empty object list of the given database kind.
func ListFor(kind string) (runtimeclient.ObjectList, error) {
	switch kind {
	case storage.PostgresDatabaseKind:
		return &storage.PostgresDatabaseList{}, nil
	case storage.MySQLDatabaseKind:
		return &storage.MySQLDatabaseList{}, nil
	}
	return nil, fmt.Errorf("unsupported database kind %q, expected one of: %s", kind, kindNames)
}

// BootstrapCompleted reports whether the bootstrap restore of the named
// database has finished. That is the restore the controller composes for a
// restoreFrom or cloneFrom reference. The database records its outcome
// durably, so it is still observed after the restore itself was
// garbage-collected.
func BootstrapCompleted(kind, name string) func(watch.Event) (bool, error) {
	return func(event watch.Event) (bool, error) {
		db, ok := event.Object.(storage.BootstrapRecorder)
		if !ok || db.GetName() != name {
			return false, nil
		}
		bootstrap := db.Bootstrap()
		if bootstrap == nil {
			return false, nil
		}
		switch bootstrap.State {
		case storage.DatabaseRestoreStateSucceeded:
			return true, nil
		case storage.DatabaseRestoreStateFailed:
			return false, fmt.Errorf(
				"%w: the restore of %s %q from its backup failed and will not be retried. Inspect it with: nctl get databaserestores %s -o yaml",
				ErrRestoreFailed, strings.ToLower(kind), name, bootstrap.Restore,
			)
		}
		return false, nil
	}
}

// ErrRestoreFailed reports a restore that reached the failed state. That
// state is final and the restore is never retried.
var ErrRestoreFailed = errors.New("restore failed")

// RestoreDone reports whether the watched DatabaseRestore has finished,
// returning an error wrapping ErrRestoreFailed if it failed.
func RestoreDone(event watch.Event) (bool, error) {
	restore, ok := event.Object.(*storage.DatabaseRestore)
	if !ok {
		return false, nil
	}
	switch restore.Status.AtProvider.State {
	case storage.DatabaseRestoreStateSucceeded:
		return true, nil
	case storage.DatabaseRestoreStateFailed:
		return false, fmt.Errorf(
			"%w: the restore %q into %s failed and will not be retried; the target database may be missing data. Inspect the restore with: nctl get databaserestores %s -o yaml",
			ErrRestoreFailed,
			restore.Name,
			FormatRef(restore.Spec.ForProvider.Target.Kind, restore.Spec.ForProvider.Target.Name),
			restore.Name,
		)
	default:
		return false, nil
	}
}
