package testutil

import (
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ManagedServer(name, namespace, location string, platform infrastructure.ManagedServerPlatform) *infrastructure.ManagedServer {
	return &infrastructure.ManagedServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: infrastructure.ManagedServerSpec{
			ForProvider: infrastructure.ManagedServerParameters{
				Location: meta.LocationName(location),
			},
		},
		Status: infrastructure.ManagedServerStatus{
			AtProvider: infrastructure.ManagedServerObservation{
				FQDN:        name,
				Platform:    platform,
				OS:          "ubuntu24.04",
				IPAddresses: []string{"192.0.2.1", "2001:db8::1"},
			},
		},
	}
}
