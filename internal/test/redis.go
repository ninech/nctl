package test

import (
	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func KeyValueStore(name, project, location string) *storage.KeyValueStore {
	return &storage.KeyValueStore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		Spec: storage.KeyValueStoreSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      name,
					Namespace: project,
				},
			},
			ForProvider: storage.KeyValueStoreParameters{
				Location: meta.LocationName(location),
			},
		},
	}
}
