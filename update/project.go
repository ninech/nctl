package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/auth"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type projectCmd struct {
	resourceCmd
	DisplayName *string `default:"" help:"Display Name of the project."`
}

func (cmd *projectCmd) Run(ctx context.Context, client *api.Client) error {
	cfg, err := auth.ReadConfig(client.KubeconfigPath, client.KubeconfigContext)
	if err != nil {
		if auth.IsConfigNotFoundError(err) {
			return auth.ReloginNeeded(err)
		}
		return err
	}

	project := &management.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: cfg.Organization,
		},
	}

	upd := newUpdater(client, project, management.ProjectKind, func(current resource.Managed) error {
		project, ok := current.(*management.Project)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, management.Project{})
		}

		cmd.applyUpdates(project)

		return nil
	})

	return upd.Update(ctx)
}

func (cmd *projectCmd) applyUpdates(project *management.Project) {
	if cmd.DisplayName != nil {
		project.Spec.DisplayName = *cmd.DisplayName
	}
}
