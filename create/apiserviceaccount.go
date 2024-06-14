package create

import (
	"context"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	iam "github.com/ninech/apis/iam/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type apiServiceAccountCmd struct {
	resourceCmd
}

func (asa *apiServiceAccountCmd) Run(ctx context.Context, client *api.Client) error {
	c := newCreator(client, asa.newAPIServiceAccount(client.Project), iam.APIServiceAccountKind)
	ctx, cancel := context.WithTimeout(ctx, asa.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !asa.Wait {
		return nil
	}

	return c.wait(ctx, waitStage{
		objectList: &iam.APIServiceAccountList{},
		onResult:   resourceAvailable,
	})
}

func (asa *apiServiceAccountCmd) newAPIServiceAccount(project string) *iam.APIServiceAccount {
	name := getName(asa.Name)
	return &iam.APIServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		Spec: iam.APIServiceAccountSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      name,
					Namespace: project,
				},
			},
		},
	}
}
