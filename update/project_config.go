package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// all fields need to be pointers so we can detect if they have been set by
// the user.
type configCmd struct {
	Size     *string            `help:"Size of the app."`
	Port     *int32             `help:"Port the app is listening on."`
	Replicas *int32             `help:"Amount of replicas of the running app."`
	Env      *map[string]string `help:"Environment variables which are passed to the app at runtime."`
}

func (cmd *configCmd) Run(ctx context.Context, client *api.Client) error {
	cfg := &apps.ProjectConfig{
		ObjectMeta: v1.ObjectMeta{
			Name:      client.Namespace,
			Namespace: client.Namespace,
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
	if cmd.Size != nil {
		newSize := apps.ApplicationSize(*cmd.Size)
		cfg.Spec.ForProvider.Config.Size = &newSize
	}
	if cmd.Port != nil {
		cfg.Spec.ForProvider.Config.Port = cmd.Port
	}
	if cmd.Replicas != nil {
		cfg.Spec.ForProvider.Config.Replicas = cmd.Replicas
	}
	if cmd.Env != nil {
		cfg.Spec.ForProvider.Config.Env = util.EnvVarsFromMap(*cmd.Env)
	}
}
