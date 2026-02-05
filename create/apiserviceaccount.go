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
	OrganizationAccess bool `help:"When enabled, this service account has access to all projects in the organization. Only valid for service accounts in the organization project."`
}

func (cmd *apiServiceAccountCmd) Run(ctx context.Context, client *api.Client) error {
	c := cmd.newCreator(client, cmd.newAPIServiceAccount(client.Project), iam.APIServiceAccountKind)
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !cmd.Wait {
		return nil
	}

	return c.wait(ctx, waitStage{
		objectList: &iam.APIServiceAccountList{},
		onResult:   resourceAvailable,
	})
}

func (cmd *apiServiceAccountCmd) newAPIServiceAccount(project string) *iam.APIServiceAccount {
	name := getName(cmd.Name)
	return &iam.APIServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		Spec: iam.APIServiceAccountSpec{
			ForProvider: iam.APIServiceAccountParameters{
				OrganizationAccess: cmd.OrganizationAccess,
				Version:            iam.APIServiceAccountV2,
			},
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      "asa-" + name,
					Namespace: project,
				},
			},
		},
	}
}
