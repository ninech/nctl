package test

import (
	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MySQL(name, project, location string) *storage.MySQL {
	return &storage.MySQL{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		Spec: storage.MySQLSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      name,
					Namespace: project,
				},
			},
			ForProvider: storage.MySQLParameters{
				Location: meta.LocationName(location),
			},
		},
	}
}
