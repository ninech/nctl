package update

import (
	"context"
	"fmt"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// all fields need to be pointers so we can detect if they have been set by
// the user.
type applicationCmd struct {
	Name           *string            `arg:"" help:"Name of the application."`
	Git            *gitConfig         `embed:"" prefix:"git-"`
	Size           *string            `help:"Size of the app."`
	Port           *int32             `help:"Port the app is listening on."`
	Replicas       *int32             `help:"Amount of replicas of the running app."`
	Hosts          *[]string          `help:"Host names where the application can be accessed. If empty, the application will just be accessible on a generated host name on the deploio.app domain."`
	BasicAuth      *bool              `help:"Enable/Disable basic authentication for the application."`
	Env            *map[string]string `help:"Environment variables which are passed to the app at runtime."`
	DeleteEnv      *[]string          `help:"Runtime environment variables names which are to be deleted."`
	BuildEnv       *map[string]string `help:"Environment variables names which are passed to the app build process."`
	DeleteBuildEnv *[]string          `help:"Build environment variables which are to be deleted."`
	DeployJob      *deployJob         `embed:"" prefix:"deploy-job-"`
}

type gitConfig struct {
	URL           *string `help:"URL to the Git repository containing the application source. Both HTTPS and SSH formats are supported."`
	SubPath       *string `help:"SubPath is a path in the git repo which contains the application code. If not given, the root directory of the git repo will be used."`
	Revision      *string `help:"Revision defines the revision of the source to deploy the application to. This can be a commit, tag or branch."`
	Username      *string `help:"Username to use when authenticating to the git repository over HTTPS." env:"GIT_USERNAME"`
	Password      *string `help:"Password to use when authenticating to the git repository over HTTPS. In case of GitHub or GitLab, this can also be an access token." env:"GIT_PASSWORD"`
	SSHPrivateKey *string `help:"Private key in x509 format to connect to the git repository via SSH." env:"GIT_SSH_PRIVATE_KEY"`
}

type deployJob struct {
	Enabled *bool          `help:"Disables the deploy job if set to false." placeholder:"false"`
	Command *string        `help:"Command to execute before a new release gets deployed. No deploy job will be executed if this is not specified." placeholder:"\"rake db:prepare\""`
	Name    *string        `help:"Name of the deploy job. The deployment will only continue if the job finished successfully." placeholder:"release"`
	Retries *int32         `help:"How many times the job will be restarted on failure." placeholder:"${app_default_deploy_job_retries}"`
	Timeout *time.Duration `help:"Timeout of the job." placeholder:"${app_default_deploy_job_timeout}"`
}

func (cmd *applicationCmd) Run(ctx context.Context, client *api.Client) error {
	// as name is a required arg this should not actually happen when called
	// through kong. But we still want to handle it in case this is called
	// directly.
	if cmd.Name == nil {
		return fmt.Errorf("name of the app has to be set")
	}

	app := &apps.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      *cmd.Name,
			Namespace: client.Project,
		},
	}

	upd := newUpdater(client, app, apps.ApplicationKind, func(current resource.Managed) error {
		app, ok := current.(*apps.Application)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, apps.Application{})
		}

		cmd.applyUpdates(app)

		if cmd.Git != nil {
			auth := util.GitAuth{
				Username:      cmd.Git.Username,
				Password:      cmd.Git.Password,
				SSHPrivateKey: cmd.Git.SSHPrivateKey,
			}

			if auth.Enabled() {
				secret := auth.Secret(app)
				if err := client.Get(ctx, client.Name(secret.Name), secret); err != nil {
					return err
				}

				auth.UpdateSecret(secret)
				if err := client.Update(ctx, secret); err != nil {
					return err
				}
			}
		}

		return nil
	})

	return upd.Update(ctx)
}

func (cmd *applicationCmd) applyUpdates(app *apps.Application) {
	if cmd.Git != nil {
		if cmd.Git.URL != nil {
			app.Spec.ForProvider.Git.URL = *cmd.Git.URL
		}
		if cmd.Git.SubPath != nil {
			app.Spec.ForProvider.Git.SubPath = *cmd.Git.SubPath
		}
		if cmd.Git.Revision != nil {
			app.Spec.ForProvider.Git.Revision = *cmd.Git.Revision
		}
	}
	if cmd.Size != nil {
		newSize := apps.ApplicationSize(*cmd.Size)
		app.Spec.ForProvider.Config.Size = newSize
	}
	if cmd.Port != nil {
		app.Spec.ForProvider.Config.Port = cmd.Port
	}
	if cmd.Replicas != nil {
		app.Spec.ForProvider.Config.Replicas = cmd.Replicas
	}
	if cmd.Hosts != nil {
		app.Spec.ForProvider.Hosts = *cmd.Hosts
	}
	if cmd.BasicAuth != nil {
		app.Spec.ForProvider.Config.EnableBasicAuth = cmd.BasicAuth
	}
	if cmd.DeployJob != nil {
		cmd.applyDeployJobUpdates(app)
	}

	// we do the nil checks inside the function, so we can execute the update
	// function only once
	app.Spec.ForProvider.Config.Env = util.UpdateEnvVars(app.Spec.ForProvider.Config.Env, cmd.Env, cmd.DeleteEnv)
	app.Spec.ForProvider.BuildEnv = util.UpdateEnvVars(app.Spec.ForProvider.BuildEnv, cmd.BuildEnv, cmd.DeleteBuildEnv)
}

func (cmd *applicationCmd) applyDeployJobUpdates(app *apps.Application) {
	if cmd.DeployJob.Enabled != nil && !*cmd.DeployJob.Enabled {
		// if enabled is explicitly set to false we set the DeployJob field to
		// nil on the API, to completely remove the object.
		app.Spec.ForProvider.Config.DeployJob = nil
		return
	}

	if cmd.DeployJob.Name != nil && len(*cmd.DeployJob.Name) != 0 {
		ensureDeployJob(app).Spec.ForProvider.Config.DeployJob.Name = *cmd.DeployJob.Name
	}
	if cmd.DeployJob.Command != nil && len(*cmd.DeployJob.Command) != 0 {
		ensureDeployJob(app).Spec.ForProvider.Config.DeployJob.Command = *cmd.DeployJob.Command
	}
	if cmd.DeployJob.Retries != nil {
		ensureDeployJob(app).Spec.ForProvider.Config.DeployJob.Retries = cmd.DeployJob.Retries
	}
	if cmd.DeployJob.Timeout != nil {
		ensureDeployJob(app).Spec.ForProvider.Config.DeployJob.Timeout = &metav1.Duration{Duration: *cmd.DeployJob.Timeout}
	}
}

func ensureDeployJob(app *apps.Application) *apps.Application {
	if app.Spec.ForProvider.Config.DeployJob == nil {
		app.Spec.ForProvider.Config.DeployJob = &apps.DeployJob{}
	}
	return app
}
