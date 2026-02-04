package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	iam "github.com/ninech/apis/iam/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type apiServiceAccountCmd struct {
	resourceCmd
	OrganizationAccess *bool `help:"When enabled, this service account has access to all projects in the organization. Only valid for service accounts in the organization project."`
}

func (cmd *apiServiceAccountCmd) Run(ctx context.Context, client *api.Client) error {
	asa := &iam.APIServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}
	return cmd.newUpdater(client, asa, iam.APIServiceAccountKind, func(current resource.Managed) error {
		asa, ok := current.(*iam.APIServiceAccount)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, iam.APIServiceAccount{})
		}
		cmd.applyUpdates(asa)
		return nil
	}).Update(ctx)
}

func (cmd *apiServiceAccountCmd) applyUpdates(asa *iam.APIServiceAccount) {
	if cmd.OrganizationAccess != nil {
		asa.Spec.ForProvider.OrganizationAccess = *cmd.OrganizationAccess
	}
}
