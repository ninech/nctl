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
	"github.com/grafana/loki/pkg/logproto"
	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/log"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/api/validation"
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
	Size                     *string           `help:"Size of the app (defaults to \"${app_default_size}\")." placeholder:"${app_default_size}"`
	Port                     *int32            `help:"Port the app is listening on (defaults to ${app_default_port})." placeholder:"${app_default_port}"`
	Replicas                 *int32            `help:"Amount of replicas of the running app (defaults to ${app_default_replicas})." placeholder:"${app_default_replicas}"`
	Hosts                    []string          `help:"Host names where the app can be accessed. If empty, the app will just be accessible on a generated host name on the deploio.app domain."`
	BasicAuth                *bool             `help:"Enable/Disable basic authentication for the app (defaults to ${app_default_basic_auth})." placeholder:"${app_default_basic_auth}"`
	Env                      map[string]string `help:"Environment variables which are passed to the app at runtime."`
	BuildEnv                 map[string]string `help:"Environment variables which are passed to the app build process."`
	DeployJob                deployJob         `embed:"" prefix:"deploy-job-"`
	WorkerJob                workerJob         `embed:"" prefix:"worker-job-"`
	GitInformationServiceURL string            `help:"URL of the git information service." default:"https://git-info.deplo.io" env:"GIT_INFORMATION_SERVICE_URL" hidden:""`
	SkipRepoAccessCheck      bool              `help:"Skip the git repository access check" default:"false"`
	Debug                    bool              `help:"Enable debug messages" default:"false"`
	Language                 string            `help:"${app_language_help} Possible values: ${enum}" enum:"ruby,php,python,golang,nodejs,static," default:""`
	DockerfileBuild          dockerfileBuild   `embed:""`
}

type gitConfig struct {
	URL                   string  `required:"" help:"URL to the Git repository containing the app source. Both HTTPS and SSH formats are supported."`
	SubPath               string  `help:"SubPath is a path in the git repo which contains the app code. If not given, the root directory of the git repo will be used."`
	Revision              string  `default:"main" help:"Revision defines the revision of the source to deploy the app to. This can be a commit, tag or branch."`
	Username              *string `help:"Username to use when authenticating to the git repository over HTTPS." env:"GIT_USERNAME"`
	Password              *string `help:"Password to use when authenticating to the git repository over HTTPS. In case of GitHub or GitLab, this can also be an access token." env:"GIT_PASSWORD"`
	SSHPrivateKey         *string `help:"Private key in PEM format to connect to the git repository via SSH." env:"GIT_SSH_PRIVATE_KEY" xor:"SSH_KEY"`
	SSHPrivateKeyFromFile *string `help:"Path to a file containing a private key in PEM format to connect to the git repository via SSH." env:"GIT_SSH_PRIVATE_KEY_FROM_FILE" xor:"SSH_KEY" predictor:"file"`
}

type deployJob struct {
	Command string        `help:"Command to execute before a new release gets deployed. No deploy job will be executed if this is not specified." placeholder:"\"rake db:prepare\""`
	Name    string        `default:"release" help:"Name of the deploy job. The deployment will only continue if the job finished successfully."`
	Retries int32         `default:"${app_default_deploy_job_retries}" help:"How many times the job will be restarted on failure. Default is ${app_default_deploy_job_retries} and maximum 5."`
	Timeout time.Duration `default:"${app_default_deploy_job_timeout}" help:"Timeout of the job. Default is ${app_default_deploy_job_timeout}, minimum is 1 minute and maximum is 30 minutes."`
}

type workerJob struct {
	Command string  `help:"Command to execute to start the worker." placeholder:"\"bundle exec sidekiq\""`
	Name    string  `help:"Name of the worker job to add." placeholder:"worker-1"`
	Size    *string `help:"Size of the worker (defaults to \"${app_default_size}\")." placeholder:"${app_default_size}"`
}

type dockerfileBuild struct {
	Enabled      bool   `name:"dockerfile" help:"${app_dockerfile_enable_help}" default:"false"`
	Path         string `name:"dockerfile-path" help:"${app_dockerfile_path_help}" default:""`
	BuildContext string `name:"dockerfile-build-context" help:"${app_dockerfile_build_context_help}" default:""`
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

const (
	buildStatusRunning = "running"
	buildStatusSuccess = "success"
	buildStatusError   = "error"
	buildStatusUnknown = "unknown"

	releaseStatusAvailable      = "available"
	releaseStatusFailure        = "failure"
	releaseStatusReplicaFailure = "replicaFailure"
)

func (app *applicationCmd) Run(ctx context.Context, client *api.Client) error {
	fmt.Println("Creating new application")
	newApp := app.newApplication(client.Project)

	sshPrivateKey, err := app.Git.sshPrivateKey()
	if err != nil {
		return fmt.Errorf("error when reading SSH private key: %w", err)
	}
	auth := util.GitAuth{
		Username:      app.Git.Username,
		Password:      app.Git.Password,
		SSHPrivateKey: sshPrivateKey,
	}

	if !app.SkipRepoAccessCheck {
		validator := &validation.RepositoryValidator{
			GitInformationServiceURL: app.GitInformationServiceURL,
			Token:                    client.Token(ctx),
			Debug:                    app.Debug,
		}
		if err := validator.Validate(ctx, &newApp.Spec.ForProvider.Git.GitTarget, auth); err != nil {
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
				if v, exists := newApp.Annotations[util.ManagedByAnnotation]; exists && v == util.NctlName {
					fmt.Println("updating git auth credentials")
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
		configValidator := &validation.ConfigValidator{
			Config: newApp.Spec.ForProvider.Config,
		}
		if err := configValidator.Validate(); err != nil {
			return fmt.Errorf("error when validating application config: %w", err)
		}
	}

	c := newCreator(client, newApp, strings.ToLower(apps.ApplicationKind))
	appWaitCtx, cancel := context.WithTimeout(ctx, app.WaitTimeout)
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

	if !app.Wait {
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
		if buildErr, ok := err.(buildError); ok {
			if err := buildErr.printMessage(printCtx, client); err != nil {
				return fmt.Errorf("%s: %w", buildErr, err)
			}
		}
		if releaseErr, ok := err.(releaseError); ok {
			if err := releaseErr.printMessage(printCtx, client); err != nil {
				return fmt.Errorf("%s: %w", releaseErr, err)
			}
		}
		return err
	}

	if err := client.Get(ctx, api.ObjectName(newApp), newApp); err != nil {
		return err
	}

	if err := spinnerMessage("CO₂ compensating the app", "🌳", 2*time.Second); err != nil {
		return err
	}

	fmt.Printf("\nYour application %q is now available at:\n  https://%s\n\n", newApp.Name, newApp.Status.AtProvider.CNAMETarget)
	printUnverifiedHostsMessage(newApp)

	basicAuthEnabled, err := latestReleaseHasBasicAuthEnabled(ctx, client, newApp)
	if err != nil {
		return fmt.Errorf("error when checking for basic-auth credentials: %w", err)
	}
	if !basicAuthEnabled {
		return nil
	}

	basicAuth, err := util.NewBasicAuthFromSecret(
		ctx,
		newApp.Status.AtProvider.BasicAuthSecret.InNamespace(newApp),
		client,
	)
	if err != nil {
		return fmt.Errorf(
			"could not gather basic auth credentials: %w\n"+
				"Please use %q to gather credentials manually",
			err,
			format.Command().GetApplication(newApp.Name, "--basic-auth-credentials"),
		)
	}
	printCredentials(basicAuth)

	return nil
}

func spinnerMessage(msg, icon string, sleepTime time.Duration) error {
	fullMsg := format.ProgressMessage(icon, msg)
	spinner, err := format.NewSpinner(fullMsg, fullMsg)
	if err != nil {
		return err
	}
	if err := spinner.Start(); err != nil {
		return err
	}
	time.Sleep(sleepTime)
	return spinner.Stop()
}

func (app *applicationCmd) config() apps.Config {
	var deployJob *apps.DeployJob

	if len(app.DeployJob.Command) != 0 && len(app.DeployJob.Name) != 0 {
		deployJob = &apps.DeployJob{
			Job: apps.Job{
				Name:    app.DeployJob.Name,
				Command: app.DeployJob.Command,
			},
			FiniteJob: apps.FiniteJob{
				Retries: ptr.To(app.DeployJob.Retries),
				Timeout: &metav1.Duration{Duration: app.DeployJob.Timeout},
			},
		}
	}

	config := apps.Config{
		EnableBasicAuth: app.BasicAuth,
		Env:             util.EnvVarsFromMap(app.Env),
		DeployJob:       deployJob,
	}

	if len(app.WorkerJob.Command) != 0 && len(app.WorkerJob.Name) != 0 {
		workerJob := apps.WorkerJob{
			Job: apps.Job{
				Name:    app.WorkerJob.Name,
				Command: app.WorkerJob.Command,
			},
		}
		if app.WorkerJob.Size != nil {
			workerJob.Size = ptr.To(apps.ApplicationSize(*app.WorkerJob.Size))
		}
		config.WorkerJobs = append(config.WorkerJobs, workerJob)
	}

	if app.Size != nil {
		config.Size = apps.ApplicationSize(*app.Size)
	}
	if app.Port != nil {
		config.Port = app.Port
	}
	if app.Replicas != nil {
		config.Replicas = app.Replicas
	}
	return config
}

func (app *applicationCmd) newApplication(project string) *apps.Application {
	name := getName(app.Name)

	return &apps.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		Spec: apps.ApplicationSpec{
			ForProvider: apps.ApplicationParameters{
				Language: apps.Language(app.Language),
				Git: apps.ApplicationGitConfig{
					GitTarget: apps.GitTarget{
						URL:      app.Git.URL,
						SubPath:  app.Git.SubPath,
						Revision: app.Git.Revision,
					},
				},
				Hosts:    app.Hosts,
				Config:   app.config(),
				BuildEnv: util.EnvVarsFromMap(app.BuildEnv),
				DockerfileBuild: apps.DockerfileBuild{
					Enabled:        app.DockerfileBuild.Enabled,
					DockerfilePath: app.DockerfileBuild.Path,
					BuildContext:   app.DockerfileBuild.BuildContext,
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

func (b buildError) printMessage(ctx context.Context, client *api.Client) error {
	fmt.Printf("\nYour build has failed with status %q. Here are the last %v lines of the log:\n\n",
		b.build.Status.AtProvider.BuildStatus, errorLogLines)
	return printBuildLogs(ctx, client, b.build)
}

func waitForBuildStart(app *apps.Application) waitStage {
	return waitStage{
		kind:       strings.ToLower(apps.BuildKind),
		objectList: &apps.BuildList{},
		listOpts: []runtimeclient.ListOption{
			runtimeclient.InNamespace(app.GetNamespace()),
			runtimeclient.MatchingLabels{util.ApplicationNameLabel: app.GetName()},
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

func waitForBuildFinish(ctx context.Context, app *apps.Application, logClient *log.Client) waitStage {
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
			runtimeclient.MatchingLabels{util.ApplicationNameLabel: app.GetName()},
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

func (r releaseError) printMessage(ctx context.Context, client *api.Client) error {
	fmt.Printf("\nYour release has failed with status %q. Here are the last %v lines of the log:\n\n",
		r.release.Status.AtProvider.ReleaseStatus, errorLogLines)
	return printReleaseLogs(ctx, client, r.release)
}

func waitForRelease(app *apps.Application) waitStage {
	return waitStage{
		kind:       strings.ToLower(apps.ReleaseKind),
		objectList: &apps.ReleaseList{},
		listOpts: []runtimeclient.ListOption{
			runtimeclient.InNamespace(app.GetNamespace()),
			runtimeclient.MatchingLabels{util.ApplicationNameLabel: app.GetName()},
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

func latestReleaseHasBasicAuthEnabled(ctx context.Context, c runtimeclient.Reader, app *apps.Application) (bool, error) {
	if app.Status.AtProvider.LatestRelease == "" {
		return false, errors.New("can not find latest release")
	}
	release := &apps.Release{}
	if err := c.Get(ctx, types.NamespacedName{Name: app.Status.AtProvider.LatestRelease, Namespace: app.Namespace}, release); err != nil {
		return false, err
	}
	if release.Spec.ForProvider.Configuration == nil {
		return false, nil
	}
	config := release.Spec.ForProvider.Configuration
	if config.EnableBasicAuth == nil {
		return false, nil
	}
	return config.EnableBasicAuth.Value, nil
}

func printUnverifiedHostsMessage(app *apps.Application) {
	unverifiedHosts := util.UnverifiedAppHosts(app)

	if len(unverifiedHosts) != 0 {
		dnsDetails := util.GatherDNSDetails([]apps.Application{*app})
		fmt.Println("You configured the following hosts:")

		for _, name := range unverifiedHosts {
			fmt.Printf("  %s\n", name)
		}

		fmt.Print("\nYour DNS details are:\n")
		fmt.Printf("  TXT record:\t%s\n", dnsDetails[0].TXTRecord)
		fmt.Printf("  DNS TARGET:\t%s\n", dnsDetails[0].CNAMETarget)

		fmt.Printf("\nTo make your app available on your custom hosts, please use \n"+
			"the DNS details and visit %s\n"+
			"for further instructions.\n",
			util.DNSSetupURL,
		)
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
		tailCtx, 0, client.Log.StdOut,
		errorLogQuery(logs.ApplicationQuery(release.Labels[util.ApplicationNameLabel], release.Namespace)),
	)
}

func printCredentials(basicAuth *util.BasicAuth) {
	fmt.Printf("\nYou can login with the following credentials:\n"+
		"  username: %s\n"+
		"  password: %s\n",
		basicAuth.Username,
		basicAuth.Password,
	)
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

	result["app_default_deploy_job_timeout"] = "5m"
	result["app_default_deploy_job_retries"] = "3"
	result["app_language_help"] = "Language specifies which language your app is. " +
		"If left empty, deploio will detect the language automatically. "
	result["app_dockerfile_enable_help"] = "Enable Dockerfile build (Beta) instead of the automatic " +
		"buildpack detection"
	result["app_dockerfile_path_help"] = "Specifies the path to the Dockerfile. If left empty a file " +
		"named Dockerfile will be searched in the application code root directory."
	result["app_dockerfile_build_context_help"] = "Defines the build context. If left empty, the application code root directory will be used as build context."
	return result, nil
}
