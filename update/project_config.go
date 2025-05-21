package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// all fields need to be pointers so we can detect if they have been set by
// the user.
type configCmd struct {
	baseConfig
}

func (cmd *configCmd) Run(ctx context.Context, client *api.Client) error {
	cfg := &apps.ProjectConfig{
		ObjectMeta: v1.ObjectMeta{
			Name:      client.Project,
			Namespace: client.Project,
		},
	}

	upd := newUpdater(client, cfg, apps.ProjectConfigKind, func(current resource.Managed) error {
		cfg, ok := current.(*apps.ProjectConfig)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, apps.ProjectConfig{})
		}

		cmd.applyUpdates(cfg)

		return nil
	})

	return upd.Update(ctx)
}

func (cmd *configCmd) applyUpdates(cfg *apps.ProjectConfig) {
	cmd.baseConfig.applyUpdates(&cfg.Spec.ForProvider.Config)
}
