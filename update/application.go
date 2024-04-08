package update

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/api/validation"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BuildTrigger is used to request a retry-build for the application.
const BuildTrigger = "BUILD_TRIGGER"

// all fields need to be pointers so we can detect if they have been set by
// the user.
type applicationCmd struct {
	Name                     *string           `arg:"" help:"Name of the application."`
	Git                      *gitConfig        `embed:"" prefix:"git-"`
	Size                     *string           `help:"Size of the app."`
	Port                     *int32            `help:"Port the app is listening on."`
	Replicas                 *int32            `help:"Amount of replicas of the running app."`
	Hosts                    *[]string         `help:"Host names where the application can be accessed. If empty, the application will just be accessible on a generated host name on the deploio.app domain."`
	BasicAuth                *bool             `help:"Enable/Disable basic authentication for the application."`
	Env                      map[string]string `help:"Environment variables which are passed to the app at runtime."`
	DeleteEnv                *[]string         `help:"Runtime environment variables names which are to be deleted."`
	BuildEnv                 map[string]string `help:"Environment variables names which are passed to the app build process."`
	DeleteBuildEnv           *[]string         `help:"Build environment variables which are to be deleted."`
	DeployJob                *deployJob        `embed:"" prefix:"deploy-job-"`
	RetryBuild               *bool             `help:"Retries build for the application if set to true." placeholder:"false"`
	GitInformationServiceURL string            `help:"URL of the git information service." default:"https://git-info.deplo.io" env:"GIT_INFORMATION_SERVICE_URL" hidden:""`
	SkipRepoAccessCheck      bool              `help:"Skip the git repository access check" default:"false"`
	Debug                    bool              `help:"Enable debug messages" default:"false"`
}

type gitConfig struct {
	URL                   *string `help:"URL to the Git repository containing the application source. Both HTTPS and SSH formats are supported."`
	SubPath               *string `help:"SubPath is a path in the git repo which contains the application code. If not given, the root directory of the git repo will be used."`
	Revision              *string `help:"Revision defines the revision of the source to deploy the application to. This can be a commit, tag or branch."`
	Username              *string `help:"Username to use when authenticating to the git repository over HTTPS." env:"GIT_USERNAME"`
	Password              *string `help:"Password to use when authenticating to the git repository over HTTPS. In case of GitHub or GitLab, this can also be an access token." env:"GIT_PASSWORD"`
	SSHPrivateKey         *string `help:"Private key in x509 format to connect to the git repository via SSH." env:"GIT_SSH_PRIVATE_KEY"`
	SSHPrivateKeyFromFile *string `help:"Path to a file containing a private key in PEM format to connect to the git repository via SSH." env:"GIT_SSH_PRIVATE_KEY_FROM_FILE" xor:"SSH_KEY"`
}

func (g gitConfig) sshPrivateKey() (*string, error) {
	if g.SSHPrivateKey != nil {
		return util.ValidatePEM(*g.SSHPrivateKey)
	}
	if g.SSHPrivateKeyFromFile == nil {
		return nil, nil
	}
	content, err := os.ReadFile(*g.SSHPrivateKeyFromFile)
	if err != nil {
		return nil, err
	}
	return util.ValidatePEM(string(content))
}

func (g gitConfig) empty() bool {
	return g.URL == nil && g.SubPath == nil &&
		g.Revision == nil && g.Username == nil &&
		g.Password == nil && g.SSHPrivateKey == nil &&
		g.SSHPrivateKeyFromFile == nil
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

		// if there was no change in the git config, we don't have
		// anything to do anymore
		if cmd.Git == nil || cmd.Git.empty() {
			return nil
		}

		sshPrivateKey, err := cmd.Git.sshPrivateKey()
		if err != nil {
			return fmt.Errorf("error when reading SSH private key: %w", err)
		}
		auth := util.GitAuth{
			Username:      cmd.Git.Username,
			Password:      cmd.Git.Password,
			SSHPrivateKey: sshPrivateKey,
		}
		if !cmd.SkipRepoAccessCheck {
			validator := &validation.RepositoryValidator{
				GitInformationServiceURL: cmd.GitInformationServiceURL,
				Token:                    client.Token,
				Debug:                    cmd.Debug,
			}

			if !auth.Enabled() {
				// if the auth was not changed but e.g. the branch changes and
				// auth is pre-configured, we need to fetch the existing git
				// auth from the app.
				a, err := util.GitAuthFromApp(ctx, client, app)
				if err != nil {
					return fmt.Errorf("error reading preconfigured auth secret")
				}
				auth = a
			}

			if err := validator.Validate(ctx, &app.Spec.ForProvider.Git.GitTarget, auth); err != nil {
				return err
			}
		}

		if auth.Enabled() {
			secret := auth.Secret(app)
			if err := client.Get(ctx, client.Name(secret.Name), secret); err != nil {
				if errors.IsNotFound(err) {
					auth.UpdateSecret(secret)
					if err := client.Create(ctx, secret); err != nil {
						return err
					}

					return nil
				}

				return err
			}

			auth.UpdateSecret(secret)
			if err := client.Update(ctx, secret); err != nil {
				return err
			}
		}

		if app.Spec.ForProvider.Config.DeployJob != nil {
			configValidator := &validation.ConfigValidator{
				Config: app.Spec.ForProvider.Config,
			}
			if err := configValidator.Validate(); err != nil {
				return fmt.Errorf("error when validating application config: %w", err)
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
		cmd.DeployJob.applyUpdates(&app.Spec.ForProvider.Config)
	}

	var delEnv []string
	if cmd.DeleteEnv != nil {
		delEnv = *cmd.DeleteEnv
	}
	app.Spec.ForProvider.Config.Env = util.UpdateEnvVars(app.Spec.ForProvider.Config.Env, cmd.Env, delEnv)

	buildEnv := make(map[string]string)
	if cmd.BuildEnv != nil {
		buildEnv = cmd.BuildEnv
	}

	if cmd.RetryBuild != nil && *cmd.RetryBuild {
		buildEnv[BuildTrigger] = time.Now().UTC().Format(time.RFC3339)
	}

	var buildDelEnv []string
	if cmd.DeleteBuildEnv != nil {
		buildDelEnv = *cmd.DeleteBuildEnv
	}
	app.Spec.ForProvider.BuildEnv = util.UpdateEnvVars(app.Spec.ForProvider.BuildEnv, buildEnv, buildDelEnv)
}

func (job deployJob) applyUpdates(cfg *apps.Config) {
	if job.Enabled != nil && !*job.Enabled {
		// if enabled is explicitly set to false we set the DeployJob field to
		// nil on the API, to completely remove the object.
		cfg.DeployJob = nil
		return
	}

	if job.Name != nil && len(*job.Name) != 0 {
		ensureDeployJob(cfg).DeployJob.Name = *job.Name
	}
	if job.Command != nil && len(*job.Command) != 0 {
		ensureDeployJob(cfg).DeployJob.Command = *job.Command
	}
	if job.Retries != nil {
		ensureDeployJob(cfg).DeployJob.Retries = job.Retries
	}
	if job.Timeout != nil {
		ensureDeployJob(cfg).DeployJob.Timeout = &metav1.Duration{Duration: *job.Timeout}
	}
}

func ensureDeployJob(cfg *apps.Config) *apps.Config {
	if cfg.DeployJob == nil {
		cfg.DeployJob = &apps.DeployJob{}
	}
	return cfg
}
