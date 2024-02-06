package delete

import (
	"context"
	"fmt"
	"time"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/auth"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type projectCmd struct {
	Name        string        `arg:"" help:"Name of the Project."`
	Force       bool          `default:"false" help:"Do not ask for confirmation of deletion."`
	Wait        bool          `default:"true" help:"Wait until Project is fully deleted"`
	WaitTimeout time.Duration `default:"5m" help:"Duration to wait for the deletion. Only relevant if wait is set."`
}

func (proj *projectCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, proj.WaitTimeout)
	defer cancel()

	cfg, err := auth.ReadConfig(client.KubeconfigPath, client.KubeconfigContext)
	if err != nil {
		if auth.IsConfigNotFoundError(err) {
			return auth.ReloginNeeded(err)
		}
		return err
	}

	d := newDeleter(
		&management.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:      proj.Name,
				Namespace: cfg.Organization,
			},
		},
		management.ProjectKind,
		prompt(projectDeletePrompt(cfg.Organization)),
	)

	// we need to overwrite the namespace as projects are always in the
	// main organization namespace
	client.Project = cfg.Organization

	if err := d.deleteResource(ctx, client, proj.WaitTimeout, proj.Wait, proj.Force); err != nil {
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
