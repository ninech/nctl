package test

import (
	management "github.com/ninech/apis/management/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Projects returns projects with the given organization namespace set
func Projects(organization string, names ...string) []client.Object {
	var projects []client.Object
	for _, name := range names {
		projects = append(projects, &management.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: organization,
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       management.ProjectKind,
				APIVersion: management.SchemeGroupVersion.String(),
			},
			Spec: management.ProjectSpec{},
		})
	}
	return projects
}
