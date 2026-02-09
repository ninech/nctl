package delete

import (
	"context"
	"fmt"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type configCmd struct {
	format.Writer `hidden:""`
	format.Reader `hidden:""`
	Force         bool          `default:"false" help:"Do not ask for confirmation of deletion."`
	Wait          bool          `default:"true" help:"Wait until Project Configuration is fully deleted."`
	WaitTimeout   time.Duration `default:"10s" help:"Duration to wait for the deletion. Only relevant if wait is set."`
}

func (cmd *configCmd) newDeleter(mg resource.Managed, kind string, opts ...deleterOption) *deleter {
	d := &deleter{
		Writer:  cmd.Writer,
		Reader:  cmd.Reader,
		kind:    kind,
		mg:      mg,
		cleanup: noCleanup,
		prompt:  defaultPrompt,
	}
	for _, opt := range opts {
		opt(d)
	}

	return d
}

func (cmd *configCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	c := &apps.ProjectConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      client.Project,
			Namespace: client.Project,
		},
	}

	d := cmd.newDeleter(c, apps.ProjectConfigKind)

	if err := d.deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force); err != nil {
		return fmt.Errorf("error while deleting %s: %w", apps.ProjectConfigKind, err)
	}

	return nil
}
