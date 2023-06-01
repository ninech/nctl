package test

import (
	"github.com/ninech/nctl/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func SetupClient(initObjs ...client.Object) (*api.Client, error) {
	scheme, err := api.NewScheme()
	if err != nil {
		return nil, err
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).Build()

	return &api.Client{
		WithWatch: client, Namespace: "default",
	}, nil
}
