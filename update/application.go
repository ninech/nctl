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
	"github.com/ninech/nctl/internal/format"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// ReleaseTrigger is used to request a new release for the application.
const ReleaseTrigger = "RELEASE_TRIGGER"

// BuildTrigger is used to request a retry-build for the application.
const BuildTrigger = "BUILD_TRIGGER"

// all fields need to be pointers so we can detect if they have been set by
// the user.
type applicationCmd struct {
	resourceCmd
	baseConfig
	Git                      *gitConfig        `embed:"" prefix:"git-"`
	Hosts                    *[]string         `help:"Host names where the application can be accessed. If empty, the application will just be accessible on a generated host name on the deploio.app domain."`
	ChangeBasicAuthPassword  *bool             `help:"Generate a new basic auth password."`
	BuildEnv                 map[string]string `help:"Environment variables names which are passed to the app build process."`
	DeleteBuildEnv           *[]string         `help:"Build environment variables which are to be deleted."`
	RetryRelease             *bool             `help:"Retries release for the application." placeholder:"false"`
	RetryBuild               *bool             `help:"Retries build for the application if set to true." placeholder:"false"`
	Pause                    *bool             `help:"Pauses the application if set to true. Stops all costs." placeholder:"false"`
	GitInformationServiceURL string            `help:"URL of the git information service." default:"https://git-info.deplo.io" env:"GIT_INFORMATION_SERVICE_URL" hidden:""`
	SkipRepoAccessCheck      bool              `help:"Skip the git repository access check" default:"false"`
	Debug                    bool              `help:"Enable debug messages" default:"false"`
	Language                 *string           `help:"${app_language_help} Possible values: ${enum}" enum:"ruby,php,python,golang,nodejs,static,"`
	DockerfileBuild          dockerfileBuild   `embed:""`
}

type baseConfig struct {
	Size      *string           `help:"Size of the app."`
	Port      *int32            `help:"Port the app is listening on."`
	Replicas  *int32            `help:"Amount of replicas of the running app."`
	BasicAuth *bool             `help:"Enable/Disable basic authentication for the application."`
	Env       map[string]string `help:"Environment variables which are passed to the app at runtime."`
	DeleteEnv *[]string         `help:"Runtime environment variables names which are to be deleted."`
	// DeployJob, ScheduledJob and WorkerJob are embedded pointers to
	// structs. Due to the usage of kong these pointers will never be `nil`.
	// So checking for `nil` values can not be used to find out if some of
	// the struct fields have been set.
	DeployJob          *deployJob    `embed:"" prefix:"deploy-job-"`
	WorkerJob          *workerJob    `embed:"" prefix:"worker-job-"`
	ScheduledJob       *scheduledJob `embed:"" prefix:"scheduled-job-"`
	DeleteWorkerJob    *string       `help:"Delete a worker job by name"`
	DeleteScheduledJob *string       `help:"Delete a scheduled job by name"`
}

type gitConfig struct {
	URL                   *string `help:"URL to the Git repository containing the application source. Both HTTPS and SSH formats are supported."`
	SubPath               *string `help:"SubPath is a path in the git repo which contains the application code. If not given, the root directory of the git repo will be used."`
	Revision              *string `help:"Revision defines the revision of the source to deploy the application to. This can be a commit, tag or branch."`
	Username              *string `help:"Username to use when authenticating to the git repository over HTTPS." env:"GIT_USERNAME"`
	Password              *string `help:"Password to use when authenticating to the git repository over HTTPS. In case of GitHub or GitLab, this can also be an access token." env:"GIT_PASSWORD"`
	SSHPrivateKey         *string `help:"Private key in x509 format to connect to the git repository via SSH." env:"GIT_SSH_PRIVATE_KEY"`
	SSHPrivateKeyFromFile *string `help:"Path to a file containing a private key in PEM format to connect to the git repository via SSH." env:"GIT_SSH_PRIVATE_KEY_FROM_FILE" xor:"SSH_KEY" predictor:"file"`
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

type workerJob struct {
	Name    *string `help:"Name of the worker job to add." placeholder:"worker-1"`
	Command *string `help:"Command to execute to start the worker." placeholder:"\"bundle exec sidekiq\""`
	Size    *string `help:"Size of the worker (defaults to \"${app_default_size}\")." placeholder:"${app_default_size}"`
}

func (wj workerJob) changesGiven() bool {
	return wj.Command != nil || wj.Size != nil
}

type scheduledJob struct {
	Command  *string        `help:"Command to execute to start the scheduled job." placeholder:"\"bundle exec rails runner\""`
	Name     *string        `help:"Name of the scheduled job job to add." placeholder:"scheduled-1"`
	Size     *string        `help:"Size (resources) of the scheduled job (defaults to \"${app_default_size}\")." placeholder:"${app_default_size}"`
	Schedule *string        `help:"Cron notation string for the scheduled job (defaults to \"* * * * *\")." placeholder:"* * * * *"`
	Retries  *int32         `help:"How many times the job will be restarted on failure." placeholder:"${app_default_scheduled_job_retries}"`
	Timeout  *time.Duration `help:"Timeout of the job." placeholder:"${app_default_scheduled_job_timeout}"`
}

func (sj scheduledJob) changesGiven() bool {
	return sj.Command != nil || sj.Size != nil || sj.Schedule != nil
}

type dockerfileBuild struct {
	Path         *string `name:"dockerfile-path" help:"${app_dockerfile_path_help}" placeholder:"."`
	BuildContext *string `name:"dockerfile-build-context" help:"${app_dockerfile_build_context_help}" placeholder:"."`
}

func (cmd *applicationCmd) Run(ctx context.Context, client *api.Client) error {
	app := &apps.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
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
				Token:                    client.Token(ctx),
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
	cmd.baseConfig.applyUpdates(&app.Spec.ForProvider.Config)

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
	if cmd.RetryRelease != nil && *cmd.RetryRelease {
		runtimeEnv := make(map[string]string)
		runtimeEnv[ReleaseTrigger] = triggerTimestamp()
		app.Spec.ForProvider.Config.Env = util.UpdateEnvVars(app.Spec.ForProvider.Config.Env, runtimeEnv, nil)
	}
	if cmd.Hosts != nil {
		app.Spec.ForProvider.Hosts = *cmd.Hosts
	}
	if cmd.ChangeBasicAuthPassword != nil {
		app.Spec.ForProvider.BasicAuthPasswordChange = ptr.To(metav1.Now())
	}
	if cmd.Language != nil {
		app.Spec.ForProvider.Language = apps.Language(*cmd.Language)
	}

	buildEnv := make(map[string]string)
	if cmd.BuildEnv != nil {
		buildEnv = cmd.BuildEnv
	}
	if cmd.RetryBuild != nil && *cmd.RetryBuild {
		buildEnv[BuildTrigger] = triggerTimestamp()
	}
	var buildDelEnv []string
	if cmd.DeleteBuildEnv != nil {
		buildDelEnv = *cmd.DeleteBuildEnv
	}
	app.Spec.ForProvider.BuildEnv = util.UpdateEnvVars(app.Spec.ForProvider.BuildEnv, buildEnv, buildDelEnv)

	if cmd.Pause != nil && *cmd.Pause {
		app.Spec.ForProvider.Paused = *cmd.Pause
	}

	if cmd.DockerfileBuild.Path != nil {
		app.Spec.ForProvider.DockerfileBuild.DockerfilePath = *cmd.DockerfileBuild.Path
		warnIfDockerfileNotEnabled(app, "path")
	}
	if cmd.DockerfileBuild.BuildContext != nil {
		app.Spec.ForProvider.DockerfileBuild.BuildContext = *cmd.DockerfileBuild.BuildContext
		warnIfDockerfileNotEnabled(app, "build context")
	}
}

func (bc baseConfig) applyUpdates(config *apps.Config) {
	if bc.BasicAuth != nil {
		config.EnableBasicAuth = bc.BasicAuth
	}
	runtimeEnv := make(map[string]string)
	if bc.Env != nil {
		runtimeEnv = bc.Env
	}
	var delEnv []string
	if bc.DeleteEnv != nil {
		delEnv = *bc.DeleteEnv
	}
	config.Env = util.UpdateEnvVars(config.Env, runtimeEnv, delEnv)

	if bc.DeployJob != nil {
		bc.DeployJob.applyUpdates(config)
	}
	if bc.WorkerJob != nil && bc.WorkerJob.changesGiven() {
		bc.WorkerJob.applyUpdates(config)
	}
	if bc.DeleteWorkerJob != nil {
		deleteWorkerJob(*bc.DeleteWorkerJob, config)
	}
	if bc.ScheduledJob != nil && bc.ScheduledJob.changesGiven() {
		bc.ScheduledJob.applyUpdates(config)
	}
	if bc.DeleteScheduledJob != nil {
		deleteScheduledJob(*bc.DeleteScheduledJob, config)
	}

	if bc.Size != nil {
		newSize := apps.ApplicationSize(*bc.Size)
		config.Size = newSize
	}
	if bc.Port != nil {
		config.Port = bc.Port
	}
	if bc.Replicas != nil {
		config.Replicas = bc.Replicas
	}
}

func triggerTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
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

func (job workerJob) applyUpdates(cfg *apps.Config) {
	if job.Name == nil {
		format.PrintWarningf("you need to pass a job name to update the command or size\n")
		return
	}
	applyToJob := func(j *apps.WorkerJob) {
		if job.Command != nil {
			j.Command = *job.Command
		}
		if job.Size != nil {
			j.Size = ptr.To(apps.ApplicationSize(*job.Size))
		}
	}
	for i := range cfg.WorkerJobs {
		if cfg.WorkerJobs[i].Name == *job.Name {
			applyToJob(&cfg.WorkerJobs[i])
			return
		}
	}

	newJob := apps.WorkerJob{Job: apps.Job{Name: *job.Name}}
	applyToJob(&newJob)
	cfg.WorkerJobs = append(cfg.WorkerJobs, newJob)
}

func deleteWorkerJob(name string, cfg *apps.Config) {
	newJobs := []apps.WorkerJob{}
	for _, wj := range cfg.WorkerJobs {
		if wj.Name != name {
			newJobs = append(newJobs, wj)
		}
	}
	if len(cfg.WorkerJobs) == len(newJobs) {
		format.PrintWarningf("did not find a worker job with the name %q\n", name)
		return
	}
	cfg.WorkerJobs = newJobs
}

func (job scheduledJob) applyUpdates(cfg *apps.Config) {
	if job.Name == nil {
		format.PrintWarningf("you need to pass a job name to update the command, schedule or size\n")
		return
	}

	applyToJob := func(j *apps.ScheduledJob) {
		if job.Command != nil {
			j.Command = *job.Command
		}
		if job.Size != nil {
			j.Size = ptr.To(apps.ApplicationSize(*job.Size))
		}
		if job.Schedule != nil {
			j.Schedule = *job.Schedule
		}
		if job.Retries != nil {
			j.Retries = job.Retries
		}
		if job.Timeout != nil {
			j.Timeout = &metav1.Duration{Duration: *job.Timeout}
		}
	}
	for i := range cfg.ScheduledJobs {
		if cfg.ScheduledJobs[i].Name == *job.Name {
			applyToJob(&cfg.ScheduledJobs[i])
			return
		}
	}

	newJob := apps.ScheduledJob{Job: apps.Job{Name: *job.Name}}
	applyToJob(&newJob)
	cfg.ScheduledJobs = append(cfg.ScheduledJobs, newJob)
}

func deleteScheduledJob(name string, cfg *apps.Config) {
	newJobs := []apps.ScheduledJob{}
	for _, sj := range cfg.ScheduledJobs {
		if sj.Name != name {
			newJobs = append(newJobs, sj)
		}
	}
	if len(cfg.ScheduledJobs) == len(newJobs) {
		format.PrintWarningf("did not find a scheduled job with the name %q\n", name)
		return
	}
	cfg.ScheduledJobs = newJobs
}

func warnIfDockerfileNotEnabled(app *apps.Application, flag string) {
	if !app.Spec.ForProvider.DockerfileBuild.Enabled {
		format.PrintWarningf("updating %s has no effect as dockerfile builds are not enabled on this app\n", flag)
	}
}
