package delete

import (
	"context"
	"fmt"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type projectCmd struct {
	resourceCmd
}

func (cmd *projectCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	org, err := client.Organization()
	if err != nil {
		return err
	}

	d := cmd.newDeleter(
		&management.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cmd.Name,
				Namespace: org,
			},
		},
		management.ProjectKind,
		prompt(projectDeletePrompt(org)),
	)

	// we need to overwrite the namespace as projects are always in the
	// main organization namespace
	client.Project = org

	if err := d.deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force); err != nil {
		return fmt.Errorf("error while deleting %s: %w", management.ProjectKind, err)
	}

	return nil
}

func projectDeletePrompt(organization string) promptFunc {
	return func(kind, name string) string {
		return fmt.Sprintf("Deleting %s %q of organization %q will also destroy all resources within this %s."+
			"\n\n !!! This can not be recovered !!! \n\n"+
			"Do you really want to continue?",
			kind,
			name,
			organization,
			kind,
		)
	}
}
