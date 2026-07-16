package test

import (
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatabaseBackup returns a DatabaseBackup of the given source database.
func DatabaseBackup(name, project string, source meta.LocalTypedReference) *storage.DatabaseBackup {
	return &storage.DatabaseBackup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		Spec: storage.DatabaseBackupSpec{
			ForProvider: storage.DatabaseBackupParameters{
				Source: source,
			},
		},
	}
}
