package delete

import (
	"context"
	"fmt"
	"time"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type configCmd struct {
	Force       bool          `default:"false" help:"Do not ask for confirmation of deletion."`
	Wait        bool          `default:"true" help:"Wait until Project Configuration is fully deleted."`
	WaitTimeout time.Duration `default:"10s" help:"Duration to wait for the deletion. Only relevant if wait is set."`
}

func (cmd *configCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	c := &apps.ProjectConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      client.Namespace,
			Namespace: client.Namespace,
		},
	}

	d := newDeleter(c, apps.ProjectConfigKind, noCleanup)

	if err := d.deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force); err != nil {
		return fmt.Errorf("error while deleting %s: %w", apps.ProjectConfigKind, err)
	}

	return nil
}
