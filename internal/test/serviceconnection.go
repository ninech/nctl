package test

import (
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	networking "github.com/ninech/apis/networking/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ServiceConnection(name, project string) *networking.ServiceConnection {
	return &networking.ServiceConnection{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		Spec: networking.ServiceConnectionSpec{
			ForProvider: networking.ServiceConnectionParameters{
				Source: networking.Source{
					Reference: meta.TypedReference{
						Reference: meta.Reference{
							Name:      "test-cluster",
							Namespace: "default",
						},
						GroupKind: metav1.GroupKind{
							Group: "infrastructure.nine.ch",
							Kind:  infrastructure.KubernetesClusterKind,
						},
					},
				},
				Destination: meta.TypedReference{
					Reference: meta.Reference{
						Name:      "test-kvs",
						Namespace: "default",
					},
					GroupKind: metav1.GroupKind{
						Group: "storage.nine.ch",
						Kind:  storage.KeyValueStoreKind,
					},
				},
			},
		},
	}
}
