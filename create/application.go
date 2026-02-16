package create

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/grafana/loki/v3/pkg/logproto"
	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/gitinfo"
	"github.com/ninech/nctl/api/log"
	"github.com/ninech/nctl/internal/application"
	"github.com/ninech/nctl/internal/cli"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/logbox"
	"github.com/ninech/nctl/logs"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/utils/ptr"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const logPrintTimeout = 10 * time.Second

// note: when adding/changing fields here also make sure to carry it over to
// update/application.go.
type applicationCmd struct {
	resourceCmd
	Git                      gitConfig         `embed:"" prefix:"git-"`
	Size                     *string           `help:"Size of the application (defaults to \"${app_default_size}\")." placeholder:"${app_default_size}"`
	Port                     *int32            `help:"Port the application is listening on (defaults to ${app_default_port})." placeholder:"${app_default_port}"`
	HealthProbe              healthProbe       `embed:"" prefix:"health-probe-"`
	Replicas                 *int32            `help:"Amount of replicas of the running application (defaults to ${app_default_replicas})." placeholder:"${app_default_replicas}"`
	Hosts                    []string          `help:"Host names where the application can be accessed. If empty, the application will just be accessible on a generated host name on the deploio.app domain."`
	BasicAuth                *bool             `help:"Enable/Disable basic authentication for the application (defaults to ${app_default_basic_auth})." placeholder:"${app_default_basic_auth}"`
	Env                      map[string]string `help:"Environment variables which are passed to the application at runtime."`
	SensitiveEnv             map[string]string `help:"Sensitive environment variables which are passed to the application at runtime."`
	BuildEnv                 map[string]string `help:"Environment variables which are passed to the application build process."`
	SensitiveBuildEnv        map[string]string `help:"Sensitive environment variables which are passed to the application build process."`
	DeployJob                deployJob         `embed:"" prefix:"deploy-job-"`
	WorkerJob                workerJob         `embed:"" prefix:"worker-job-"`
	ScheduledJob             scheduledJob      `embed:"" prefix:"scheduled-job-"`
	GitInformationServiceURL string            `help:"URL of the git information service." default:"https://git-info.deplo.io" env:"GIT_INFORMATION_SERVICE_URL" hidden:""`
	SkipRepoAccessCheck      bool              `help:"Skip the git repository access check." default:"false"`
	Debug                    bool              `help:"Enable debug messages." default:"false"`
	Language                 string            `help:"${app_language_help} Possible values: ${enum}" enum:"ruby,php,python,golang,nodejs,static," default:""`
	DockerfileBuild          dockerfileBuild   `embed:""`
}

type gitConfig struct {
	URL                   string  `required:"" help:"URL to the Git repository containing the application source. Both HTTPS and SSH formats are supported."`
	SubPath               string  `help:"SubPath is a path in the git repository which contains the application code. If not given, the root directory of the git repository will be used."`
	Revision              string  `default:"main" help:"Revision defines the revision of the source to deploy the application to. This can be a commit, tag or branch."`
	Username              *string `help:"Username to use when authenticating to the git repository over HTTPS." env:"GIT_USERNAME"`
	Password              *string `help:"Password to use when authenticating to the git repository over HTTPS. In case of GitHub or GitLab, this can also be an access token." env:"GIT_PASSWORD"`
	SSHPrivateKey         *string `help:"Private key in PEM format to connect to the git repository via SSH." env:"GIT_SSH_PRIVATE_KEY" xor:"SSH_KEY"`
	SSHPrivateKeyFromFile *string `help:"Path to a file containing a private key in PEM format to connect to the git repository via SSH." env:"GIT_SSH_PRIVATE_KEY_FROM_FILE" xor:"SSH_KEY" completion-predictor:"file"`
}

type healthProbe struct {
	PeriodSeconds int32  `placeholder:"${app_default_health_probe_period_seconds}" help:"How often (in seconds) to perform the custom health probe. Minimum is 1." default:"${app_default_health_probe_period_seconds}"`
	Path          string `help:"URL path on the application's HTTP server used for the custom health probe. The platform performs an HTTP GET on this path to determine health. The app should return a non-success status during startup and once healthy, return a success HTTP status. Any code 200-399 indicates success; any other code indicates failure."`
}

type deployJob struct {
	Command string        `help:"Command to execute before a new release gets deployed. No deploy job will be executed if this is not specified." placeholder:"\"rake db:prepare\""`
	Name    string        `default:"release" help:"Name of the deploy job. The deployment will only continue if the job finished successfully."`
	Retries int32         `default:"${app_default_deploy_job_retries}" help:"How many times the job will be restarted on failure. Default is ${app_default_deploy_job_retries} and maximum 5."`
	Timeout time.Duration `default:"${app_default_deploy_job_timeout}" help:"Timeout of the job. Default is ${app_default_deploy_job_timeout}, minimum is 1 minute and maximum is 30 minutes."`
}

type workerJob struct {
	Command string  `help:"Command to execute to start the worker job." placeholder:"\"bundle exec sidekiq\""`
	Name    string  `help:"Name of the worker job to add." placeholder:"worker-1"`
	Size    *string `help:"Size of the worker job (defaults to \"${app_default_size}\")." placeholder:"${app_default_size}"`
}

type scheduledJob struct {
	Command  string        `help:"Command to execute to start the scheduled job." placeholder:"\"bundle exec rails runner\""`
	Name     string        `help:"Name of the scheduled job job to add." placeholder:"scheduled-1"`
	Size     *string       `help:"Size (resources) of the scheduled job (defaults to \"${app_default_size}\")." placeholder:"${app_default_size}"`
	Schedule string        `help:"Cron notation string for the scheduled job (defaults to \"* * * * *\")." placeholder:"* * * * *"`
	Retries  int32         `default:"${app_default_scheduled_job_retries}" help:"How many times the job will be restarted on failure. Default is ${app_default_scheduled_job_retries} and maximum 5."`
	Timeout  time.Duration `default:"${app_default_scheduled_job_timeout}" help:"Timeout of the job. Default is ${app_default_scheduled_job_timeout}, minimum is 1 minute and maximum is 30 minutes."`
}

type dockerfileBuild struct {
	Enabled      bool   `name:"dockerfile" help:"${app_dockerfile_enable_help}." default:"false"`
	Path         string `name:"dockerfile-path" help:"${app_dockerfile_path_help}." default:""`
	BuildContext string `name:"dockerfile-build-context" help:"${app_dockerfile_build_context_help}." default:""`
}

func (g gitConfig) sshPrivateKey() (*string, error) {
	if g.SSHPrivateKey != nil {
		return application.ValidatePEM(*g.SSHPrivateKey)
	}
	if g.SSHPrivateKeyFromFile == nil {
		return nil, nil
	}
	content, err := os.ReadFile(*g.SSHPrivateKeyFromFile)
	if err != nil {
		return nil, err
	}
	return application.ValidatePEM(string(content))
}

const (
	buildStatusRunning = "running"
	buildStatusSuccess = "success"
	buildStatusError   = "error"
	buildStatusUnknown = "unknown"

	releaseStatusAvailable      = "available"
	releaseStatusFailure        = "failure"
	releaseStatusReplicaFailure = "replicaFailure"
)

func (cmd *applicationCmd) Run(ctx context.Context, client *api.Client) error {
	newApp := cmd.newApplication(client.Project)

	sshPrivateKey, err := cmd.Git.sshPrivateKey()
	if err != nil {
		return fmt.Errorf("error when reading SSH private key: %w", err)
	}
	auth := gitinfo.Auth{
		Username:      cmd.Git.Username,
		Password:      cmd.Git.Password,
		SSHPrivateKey: sshPrivateKey,
	}

	if !cmd.SkipRepoAccessCheck {
		client, err := gitinfo.New(cmd.GitInformationServiceURL, client.Token(ctx))
		if err != nil {
			return err
		}

		validator := &application.RepositoryValidator{
			Auth:   auth,
			Client: client,
			Debug:  cmd.Debug,
		}
		newApp.Spec.ForProvider.Git.GitTarget, err = validator.Validate(ctx, newApp.Spec.ForProvider.Git.GitTarget)
		if err != nil {
			return err
		}
	}

	if auth.Enabled() {
		if err := auth.Valid(); err != nil {
			return fmt.Errorf("the credentials are given but they are empty: %w", err)
		}

		secret := auth.Secret(newApp)
		// for git auth we create a separate secret and then reference it in the app.
		if err := client.Create(ctx, secret); err != nil {
			if kerrors.IsAlreadyExists(err) {
				// only update the secret if it is managed by nctl in the first place
				if cli.IsManagedBy(newApp.Annotations) {
					cmd.Successf("🔐", "updating git auth credentials")
					if err := client.Get(ctx, client.Name(secret.Name), secret); err != nil {
						return err
					}

					auth.UpdateSecret(secret)
					if err := client.Update(ctx, secret); err != nil {
						return err
					}
				}
			} else {
				return fmt.Errorf("unable to create git auth secret: %w", err)
			}
		}

		newApp.Spec.ForProvider.Git.Auth = &apps.GitAuth{
			FromSecret: &meta.LocalReference{
				Name: secret.GetName(),
			},
		}
	}

	if newApp.Spec.ForProvider.Config.DeployJob != nil {
		if err := application.ValidateConfig(newApp.Spec.ForProvider.Config); err != nil {
			return fmt.Errorf("error when validating application config: %w", err)
		}
	}

	c := cmd.newCreator(client, newApp, apps.ApplicationKind)
	appWaitCtx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(appWaitCtx); err != nil {
		if auth.Enabled() {
			secret := auth.Secret(newApp)
			if gitErr := client.Delete(ctx, secret); gitErr != nil {
				return errors.Join(err, fmt.Errorf("unable to delete git auth secret: %w", gitErr))
			}
		}

		return err
	}

	if !cmd.Wait {
		return nil
	}

	if err := c.wait(
		appWaitCtx,
		waitForBuildStart(newApp),
		waitForBuildFinish(appWaitCtx, newApp, client.Log),
		waitForRelease(newApp),
	); err != nil {
		printCtx, cancel := context.WithTimeout(context.Background(), logPrintTimeout)
		defer cancel()
		if printErr := cmd.printErrorDetails(printCtx, client, err); printErr != nil {
			return fmt.Errorf("%s: %w", err, printErr)
		}
		return err
	}

	if err := client.Get(ctx, api.ObjectName(newApp), newApp); err != nil {
		return err
	}

	if err := cmd.spinnerMessage("CO₂ compensating the app", "🌳", 2*time.Second); err != nil {
		return err
	}

	cmd.Successf(
		"🚀",
		"Your application %q is now available at:\n  https://%s",
		newApp.Name,
		newApp.Status.AtProvider.CNAMETarget,
	)
	cmd.printUnverifiedHostsMessage(newApp)

	basicAuthEnabled, err := latestReleaseHasBasicAuthEnabled(ctx, client, newApp)
	if err != nil {
		return fmt.Errorf("error when checking for basic-auth credentials: %w", err)
	}
	if !basicAuthEnabled {
		return nil
	}

	basicAuth, err := application.NewBasicAuthFromSecret(
		ctx,
		newApp.Status.AtProvider.BasicAuthSecret.InNamespace(newApp),
		client,
	)
	if err != nil {
		return fmt.Errorf(
			"could not gather basic auth credentials: %w\n"+
				"Please use %q to gather credentials manually",
			err,
			format.Command().Get(apps.ApplicationKind, newApp.Name, "--basic-auth-credentials"),
		)
	}
	cmd.printCredentials(basicAuth)

	return nil
}

func (cmd *applicationCmd) spinnerMessage(msg, icon string, sleepTime time.Duration) error {
	fullMsg := format.Progress(icon, msg)
	spinner, err := cmd.Spinner(fullMsg, fullMsg)
	if err != nil {
		return err
	}
	if err := spinner.Start(); err != nil {
		return err
	}
	time.Sleep(sleepTime)
	return spinner.Stop()
}

func combineEnvVars(plain, sensitive map[string]string) apps.EnvVars {
	return append(
		application.EnvVarsFromMap(plain),
		application.EnvVarsFromMap(sensitive, application.Sensitive())...,
	)
}

func (cmd *applicationCmd) config() apps.Config {
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
	config := apps.Config{
		EnableBasicAuth: cmd.BasicAuth,
		Env:             combineEnvVars(cmd.Env, cmd.SensitiveEnv),
		DeployJob:       deployJob,
	}

	if len(cmd.WorkerJob.Command) != 0 && len(cmd.WorkerJob.Name) != 0 {
		workerJob := apps.WorkerJob{
			Job: apps.Job{
				Name:    cmd.WorkerJob.Name,
				Command: cmd.WorkerJob.Command,
			},
		}
		if cmd.WorkerJob.Size != nil {
			workerJob.Size = ptr.To(apps.ApplicationSize(*cmd.WorkerJob.Size))
		}
		config.WorkerJobs = append(config.WorkerJobs, workerJob)
	}

	if len(cmd.ScheduledJob.Command) != 0 && len(cmd.ScheduledJob.Name) != 0 &&
		len(cmd.ScheduledJob.Schedule) != 0 {
		scheduledJob := apps.ScheduledJob{
			FiniteJob: apps.FiniteJob{
				Retries: &cmd.ScheduledJob.Retries,
				Timeout: &metav1.Duration{Duration: cmd.ScheduledJob.Timeout},
			},
			Job: apps.Job{
				Name:    cmd.ScheduledJob.Name,
				Command: cmd.ScheduledJob.Command,
			},
			Schedule: cmd.ScheduledJob.Schedule,
		}
		if cmd.ScheduledJob.Size != nil {
			scheduledJob.Size = ptr.To(apps.ApplicationSize(*cmd.ScheduledJob.Size))
		}
		config.ScheduledJobs = append(config.ScheduledJobs, scheduledJob)
	}

	if cmd.Size != nil {
		config.Size = apps.ApplicationSize(*cmd.Size)
	}
	if cmd.Port != nil {
		config.Port = cmd.Port
	}
	if cmd.Replicas != nil {
		config.Replicas = cmd.Replicas
	}

	cmd.HealthProbe.applyCreate(&config)

	return config
}

func (h healthProbe) ToProbePatch() application.ProbePatch {
	var pp application.ProbePatch

	if p := strings.TrimSpace(h.Path); p != "" {
		pp.Path = application.OptString{State: application.Set, Val: p}
	}
	if h.PeriodSeconds > 0 {
		pp.PeriodSeconds = application.OptInt32{State: application.Set, Val: h.PeriodSeconds}
	}
	return pp
}

func (h healthProbe) applyCreate(cfg *apps.Config) {
	application.ApplyProbePatch(cfg, h.ToProbePatch())
}

func (cmd *applicationCmd) newApplication(project string) *apps.Application {
	name := getName(cmd.Name)

	return &apps.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		Spec: apps.ApplicationSpec{
			ForProvider: apps.ApplicationParameters{
				Language: apps.Language(cmd.Language),
				Git: apps.ApplicationGitConfig{
					GitTarget: apps.GitTarget{
						URL:      cmd.Git.URL,
						SubPath:  cmd.Git.SubPath,
						Revision: cmd.Git.Revision,
					},
				},
				Hosts:    cmd.Hosts,
				Config:   cmd.config(),
				BuildEnv: combineEnvVars(cmd.BuildEnv, cmd.SensitiveBuildEnv),
				DockerfileBuild: apps.DockerfileBuild{
					Enabled:        cmd.DockerfileBuild.Enabled,
					DockerfilePath: cmd.DockerfileBuild.Path,
					BuildContext:   cmd.DockerfileBuild.BuildContext,
				},
			},
		},
	}
}

type buildError struct {
	build *apps.Build
}

func (b buildError) Error() string {
	return fmt.Sprintf("build failed with status %s", b.build.Status.AtProvider.BuildStatus)
}

// Build returns the build that caused the error.
func (b buildError) Build() *apps.Build { return b.build }

func waitForBuildStart(app *apps.Application) waitStage {
	return waitStage{
		kind:       strings.ToLower(apps.BuildKind),
		objectList: &apps.BuildList{},
		listOpts: []runtimeclient.ListOption{
			runtimeclient.InNamespace(app.GetNamespace()),
			runtimeclient.MatchingLabels{application.ApplicationNameLabel: app.GetName()},
		},
		waitMessage: &message{
			text: "waiting for build to start",
			icon: "⏳",
		},
		doneMessage: &message{
			disabled: true,
		},
		onResult: func(e watch.Event) (bool, error) {
			build, ok := e.Object.(*apps.Build)
			if !ok {
				return false, nil
			}

			switch build.Status.AtProvider.BuildStatus {
			case buildStatusRunning:
				return true, nil
			case buildStatusError:
				fallthrough
			case buildStatusUnknown:
				return false, buildError{build: build}
			}

			return false, nil
		},
	}
}

func waitForBuildFinish(
	ctx context.Context,
	app *apps.Application,
	logClient *log.Client,
) waitStage {
	msg := message{icon: "📦", text: "building application"}
	p := tea.NewProgram(
		logbox.New(15, msg.progress()),
		tea.WithoutSignalHandler(),
		tea.WithInput(nil),
		tea.WithContext(ctx),
	)
	return waitStage{
		disableSpinner: true,
		kind:           strings.ToLower(apps.BuildKind),
		objectList:     &apps.BuildList{},
		listOpts: []runtimeclient.ListOption{
			runtimeclient.InNamespace(app.GetNamespace()),
			runtimeclient.MatchingLabels{application.ApplicationNameLabel: app.GetName()},
		},
		waitMessage: nil,
		doneMessage: &message{
			disabled: true,
		},
		beforeWait: func() {
			// setup the log tailing and send it to the logbox. Run in the
			// background until the context is cancelled.
			go func() {
				if err := logClient.TailQuery(
					ctx, 0, &logbox.Output{Program: p},
					log.Query{
						QueryString: logs.BuildsOfAppQuery(app.Name, app.Namespace),
						Limit:       10,
						Start:       time.Now(),
						End:         time.Now(),
						Direction:   logproto.BACKWARD,
						Quiet:       true,
					},
				); err != nil {
					fmt.Fprintf(os.Stderr, "error tailing the build log: %s", err)
				}
			}()

			go func() {
				if _, err := p.Run(); err != nil {
					if !errors.Is(err, tea.ErrProgramKilled) {
						fmt.Fprintf(os.Stderr, "error running tea program: %s", err)
					}
				}
			}()
		},
		afterWait: func() {
			// ensure to cleanly shutdown the tea program
			p.Quit()
			p.Wait()
		},
		onResult: func(e watch.Event) (bool, error) {
			build, ok := e.Object.(*apps.Build)
			if !ok {
				return false, nil
			}

			switch build.Status.AtProvider.BuildStatus {
			case buildStatusSuccess:
				p.Send(logbox.Msg{Done: true})
				return true, nil
			case buildStatusError:
				fallthrough
			case buildStatusUnknown:
				p.Send(logbox.Msg{Done: true})
				return false, buildError{build: build}
			}

			return false, nil
		},
	}
}

type releaseError struct {
	release *apps.Release
}

func (r releaseError) Error() string {
	deployJobStatus := r.release.Status.AtProvider.DeployJobStatus
	if deployJobStatus != nil && deployJobStatus.Status == "" {
		switch deployJobStatus.Reason {
		case apps.DeployJobProcessReasonBackoff:
			return "deploy job has failed after all retries."
		case apps.DeployJobProcessReasonTimeout:
			return "deploy job of release timed out. Increase the timeout or ensure it finishes earlier."
		}
	}
	return fmt.Sprintf("release failed with status %s", r.release.Status.AtProvider.ReleaseStatus)
}

// Release returns the release that caused the error.
func (r releaseError) Release() *apps.Release { return r.release }

func waitForRelease(app *apps.Application) waitStage {
	return waitStage{
		kind:       strings.ToLower(apps.ReleaseKind),
		objectList: &apps.ReleaseList{},
		listOpts: []runtimeclient.ListOption{
			runtimeclient.InNamespace(app.GetNamespace()),
			runtimeclient.MatchingLabels{application.ApplicationNameLabel: app.GetName()},
		},
		waitMessage: &message{
			text: "releasing application",
			icon: "🚦",
		},
		doneMessage: &message{
			text: "release available",
			icon: "⛺",
		},
		onResult: func(e watch.Event) (bool, error) {
			release, ok := e.Object.(*apps.Release)
			if !ok {
				return false, nil
			}

			switch release.Status.AtProvider.ReleaseStatus {
			case releaseStatusAvailable:
				return true, nil
			case releaseStatusFailure, releaseStatusReplicaFailure:
				return false, releaseError{release: release}
			}

			return false, nil
		},
	}
}

func latestReleaseHasBasicAuthEnabled(
	ctx context.Context,
	c runtimeclient.Reader,
	app *apps.Application,
) (bool, error) {
	if app.Status.AtProvider.LatestRelease == "" {
		return false, errors.New("can not find latest release")
	}
	release := &apps.Release{}
	if err := c.Get(
		ctx,
		types.NamespacedName{Name: app.Status.AtProvider.LatestRelease, Namespace: app.Namespace},
		release,
	); err != nil {
		return false, err
	}
	config := release.Spec.ForProvider.Configuration
	if config.EnableBasicAuth == nil {
		return false, nil
	}
	return config.EnableBasicAuth.Value, nil
}

func (cmd *applicationCmd) printUnverifiedHostsMessage(app *apps.Application) {
	unverifiedHosts := application.UnverifiedHosts(app)

	if len(unverifiedHosts) != 0 {
		dnsDetails := application.DNSDetails([]apps.Application{*app})
		cmd.Infof("🌐", "You configured the following hosts:")

		for _, name := range unverifiedHosts {
			cmd.Printf("  %s\n", name)
		}

		cmd.Infof("📋", "Your DNS details are:")
		cmd.Printf("  TXT record:\t%s\n", dnsDetails[0].TXTRecord)
		cmd.Printf("  DNS TARGET:\t%s\n", dnsDetails[0].CNAMETarget)

		cmd.Infof("ℹ", "To make your app available on your custom hosts, please use \nthe DNS details and visit %s\nfor further instructions.", application.DNSSetupURL)
	}
}

func printBuildLogs(ctx context.Context, client *api.Client, build *apps.Build) error {
	tailCtx, cancel := context.WithTimeout(ctx, errorTailTimeout)
	defer cancel()
	return client.Log.TailQuery(
		tailCtx, 0, client.Log.StdOut,
		errorLogQuery(logs.BuildQuery(build.Name, build.Namespace)),
	)
}

func printReleaseLogs(ctx context.Context, client *api.Client, release *apps.Release) error {
	tailCtx, cancel := context.WithTimeout(ctx, errorTailTimeout)
	defer cancel()
	return client.Log.TailQuery(
		tailCtx,
		0,
		client.Log.StdOut,
		errorLogQuery(
			logs.ApplicationQuery(release.Labels[application.ApplicationNameLabel], release.Namespace),
		),
	)
}

func (cmd *applicationCmd) printCredentials(basicAuth *application.BasicAuth) {
	cmd.Infof("🔐", "You can login with the following credentials:")
	cmd.Printf("  username: %s\n  password: %s\n", basicAuth.Username, basicAuth.Password)
}

// printErrorDetails prints detailed error information for build and release errors.
func (cmd *applicationCmd) printErrorDetails(
	ctx context.Context,
	client *api.Client,
	err error,
) error {
	var buildErr buildError
	if errors.As(err, &buildErr) {
		cmd.Infof("❌", "Your build has failed with status %q. Here are the last %v lines of the log:",
			buildErr.Build().Status.AtProvider.BuildStatus,
			errorLogLines,
		)
		return printBuildLogs(ctx, client, buildErr.Build())
	}

	var releaseErr releaseError
	if errors.As(err, &releaseErr) {
		cmd.Infof("❌", "Your release has failed with status %q. Here are the last %v lines of the log:",
			releaseErr.Release().Status.AtProvider.ReleaseStatus,
			errorLogLines,
		)
		return printReleaseLogs(ctx, client, releaseErr.Release())
	}

	return nil
}

// we print the last 40 lines of the log. In most cases this should be
// enough to give a hint about the problem but we might need to tweak this
// value a bit in the future.
const errorLogLines = 40

// when we print error logs, we want to tail the log for a bit as new data might
// still be coming in even after the build/release already has failed.
const errorTailTimeout = 5 * time.Second

func errorLogQuery(queryString string) log.Query {
	return log.Query{
		QueryString: queryString,
		Limit:       errorLogLines,
		Start:       time.Now().Add(-time.Hour),
		End:         time.Now(),
		Direction:   logproto.BACKWARD,
		Quiet:       true,
	}
}

// ApplicationKongVars returns all variables which are used in the application
// create command
func ApplicationKongVars() (kong.Vars, error) {
	result := make(kong.Vars)
	result["app_default_size"] = string(apps.DefaultConfig.Size)
	if apps.DefaultConfig.Port == nil {
		return nil, errors.New("no default application port found")
	}
	result["app_default_port"] = strconv.Itoa(int(*apps.DefaultConfig.Port))
	if apps.DefaultConfig.Replicas == nil {
		return nil, errors.New("no default application replicas found")
	}
	result["app_default_replicas"] = strconv.Itoa(int(*apps.DefaultConfig.Replicas))
	if apps.DefaultConfig.EnableBasicAuth == nil {
		return nil, errors.New("no default application basic authentication settings found")
	}
	result["app_default_basic_auth"] = strconv.FormatBool(*apps.DefaultConfig.EnableBasicAuth)

	result["app_default_health_probe_period_seconds"] = "10"

	result["app_default_deploy_job_timeout"] = "5m"
	result["app_default_deploy_job_retries"] = "3"
	result["app_default_scheduled_job_timeout"] = "5m"
	result["app_default_scheduled_job_retries"] = "0"
	result["app_language_help"] = "Language specifies which language your app is. " +
		"If left empty, deploio will detect the language automatically. "
	result["app_dockerfile_enable_help"] = "Enable Dockerfile build (Beta) instead of the automatic " +
		"buildpack detection"
	result["app_dockerfile_path_help"] = "Specifies the path to the Dockerfile. If left empty a file " +
		"named Dockerfile will be searched in the application code root directory."
	result["app_dockerfile_build_context_help"] = "Defines the build context. If left empty, the application code root directory will be used as build context."
	return result, nil
}
