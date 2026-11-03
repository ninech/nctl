// Package database contains helpers shared by the database resource commands
package database

import (
	"errors"
	"fmt"
	"slices"
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

// LocalRef returns ref as a reference to a database in the same project. It
// returns an error if ref is not of a database kind.
func LocalRef(ref meta.TypedReference) (meta.LocalTypedReference, error) {
	if !slices.Contains(kinds, ref.Kind) {
		return meta.LocalTypedReference{}, unsupportedKind(ref.Kind)
	}
	return Ref(ref.Kind, ref.Name), nil
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
	return nil, unsupportedKind(kind)
}

func unsupportedKind(kind string) error {
	return fmt.Errorf("unsupported database kind %q, expected one of: %s", strings.ToLower(kind), kindNames)
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
// target, with a server-generated suffix. All other fields are defaulted
// server-side.
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
	target := FormatRef(restore.Spec.ForProvider.Target.Kind, restore.Spec.ForProvider.Target.Name)
	switch restore.Status.AtProvider.State {
	case storage.DatabaseJobStateSucceeded:
		return true, nil
	case storage.DatabaseJobStateFailed:
		return false, fmt.Errorf(
			"%w: the restore %q into %s failed and will not be retried; the target database may be missing data. Inspect the restore with: nctl get databaserestores %s -o yaml",
			ErrRestoreFailed,
			restore.Name,
			target,
			restore.Name,
		)
	}
	// A deleted restore sends no further events, so the wait would only end
	// at its timeout.
	if event.Type == watch.Deleted {
		return false, fmt.Errorf("the restore %q into %s was deleted before it finished, check the data of %s",
			restore.Name, target, target)
	}
	return false, nil
}
