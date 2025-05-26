package create

import (
	"context"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// all fields need to be pointers so we can detect if they have been set by
// the user.
type configCmd struct {
	baseConfig
}

func (cmd *configCmd) Run(ctx context.Context, client *api.Client) error {
	c := newCreator(client, cmd.newProjectConfig(client.Project), apps.ProjectConfigGroupKind)

	return c.createResource(ctx)
}

func (cmd *configCmd) newProjectConfig(namespace string) *apps.ProjectConfig {
	return &apps.ProjectConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespace,
			Namespace: namespace,
		},
		Spec: apps.ProjectConfigSpec{
			ForProvider: apps.ProjectConfigParameters{
				Config: newConfig(cmd.baseConfig),
			},
		},
	}
}
