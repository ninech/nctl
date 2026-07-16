package test

import (
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatabaseRestore returns a DatabaseRestore of the named backup into the
// given target database.
func DatabaseRestore(name, project, backup string, target meta.LocalTypedReference) *storage.DatabaseRestore {
	return &storage.DatabaseRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		Spec: storage.DatabaseRestoreSpec{
			ForProvider: storage.DatabaseRestoreParameters{
				Backup: meta.LocalReference{Name: backup},
				Target: target,
			},
		},
	}
}
