package create

import (
	"context"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// all fields need to be pointers so we can detect if they have been set by
// the user.
type configCmd struct {
	Size      string             `help:"Size of the app."`
	Port      *int32             `help:"Port the app is listening on."`
	Replicas  *int32             `help:"Amount of replicas of the running app."`
	Env       *map[string]string `help:"Environment variables which are passed to the app at runtime."`
	BasicAuth *bool              `help:"Enable/Disable basic authentication for applications."`
}

func (cmd *configCmd) Run(ctx context.Context, client *api.Client) error {
	c := newCreator(client, cmd.newProjectConfig(client.Project), apps.ProjectConfigGroupKind)

	return c.createResource(ctx)
}

func (cmd *configCmd) newProjectConfig(namespace string) *apps.ProjectConfig {
	env := apps.EnvVars{}
	if cmd.Env != nil {
		env = util.EnvVarsFromMap(*cmd.Env)
	}

	return &apps.ProjectConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespace,
			Namespace: namespace,
		},
		Spec: apps.ProjectConfigSpec{
			ForProvider: apps.ProjectConfigParameters{
				Config: apps.Config{
					Size:            apps.ApplicationSize(cmd.Size),
					Replicas:        cmd.Replicas,
					Port:            cmd.Port,
					Env:             env,
					EnableBasicAuth: cmd.BasicAuth,
				},
			},
		},
	}
}
