package test

import (
	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	observability "github.com/ninech/apis/observability/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Grafana(name, project string) *observability.Grafana {
	return &observability.Grafana{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		Spec: observability.GrafanaSpec{
			ResourceSpec: runtimev1.ResourceSpec{},
			ForProvider:  observability.GrafanaParameters{},
		},
	}
}
