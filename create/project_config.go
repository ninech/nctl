package create

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/format"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// all fields need to be pointers so we can detect if they have been set by
// the user.
type configCmd struct {
	format.Writer

	Size      string             `help:"Size of the application."`
	Port      *int32             `help:"Port the application is listening on."`
	Replicas  *int32             `help:"Amount of replicas of the running application."`
	Env       *map[string]string `help:"Environment variables which are passed to the application at runtime."`
	BasicAuth *bool              `help:"Enable/Disable basic authentication for applications."`
	DeployJob deployJob          `embed:"" prefix:"deploy-job-"`
}

func (cmd *configCmd) newCreator(
	client *api.Client,
	mg resource.Managed,
	resourceName string,
) *creator {
	return &creator{client: client, mg: mg, kind: resourceName, Writer: cmd.Writer}
}

func (cmd *configCmd) Run(ctx context.Context, client *api.Client) error {
	c := cmd.newCreator(client, cmd.newProjectConfig(client.Project), apps.ProjectConfigGroupKind)

	return c.createResource(ctx)
}

func (cmd *configCmd) newProjectConfig(namespace string) *apps.ProjectConfig {
	env := apps.EnvVars{}
	if cmd.Env != nil {
		env = util.EnvVarsFromMap(*cmd.Env)
	}

	var deployJob *apps.DeployJob
	if len(cmd.DeployJob.Command) != 0 && len(cmd.DeployJob.Name) != 0 {
		deployJob = &apps.DeployJob{
			Job: apps.Job{
				Name:    cmd.DeployJob.Name,
				Command: cmd.DeployJob.Command,
			},
			FiniteJob: apps.FiniteJob{
				Retries: ptr.To(cmd.DeployJob.Retries),
				Timeout: &metav1.Duration{Duration: cmd.DeployJob.Timeout},
			},
		}
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
					DeployJob:       deployJob,
				},
			},
		},
	}
}
