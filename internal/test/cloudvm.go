package test

import (
	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CloudVirtualMachine(name, namespace, location string, powerstate infrastructure.VirtualMachinePowerState) *infrastructure.CloudVirtualMachine {
	return &infrastructure.CloudVirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: infrastructure.CloudVirtualMachineSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      "cloudvirtualmachine-" + name,
					Namespace: namespace,
				},
			},
			ForProvider: infrastructure.CloudVirtualMachineParameters{
				Location:   meta.LocationName(location),
				PowerState: powerstate,
			},
		},
	}
}
