package test

import (
	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	apps "github.com/ninech/apis/apps/v1alpha1"
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	networking "github.com/ninech/apis/networking/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func StaticEgress(name, project, appName string) *networking.StaticEgress {
	return &networking.StaticEgress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		Spec: networking.StaticEgressSpec{
			ResourceSpec: runtimev1.ResourceSpec{},
			ForProvider: networking.StaticEgressParameters{
				Target: meta.LocalTypedReference{
					LocalReference: meta.LocalReference{
						Name: appName,
					},
					GroupKind: metav1.GroupKind{
						Group: apps.Group,
						Kind:  apps.ApplicationKind,
					},
				},
			},
		},
	}
}

func StaticEgressForCluster(name, project, clusterName string) *networking.StaticEgress {
	return &networking.StaticEgress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		Spec: networking.StaticEgressSpec{
			ResourceSpec: runtimev1.ResourceSpec{},
			ForProvider: networking.StaticEgressParameters{
				Target: meta.LocalTypedReference{
					LocalReference: meta.LocalReference{
						Name: clusterName,
					},
					GroupKind: metav1.GroupKind{
						Group: infrastructure.Group,
						Kind:  infrastructure.KubernetesClusterKind,
					},
				},
			},
		},
	}
}
