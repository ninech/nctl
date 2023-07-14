package create

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"github.com/grafana/loki/pkg/logproto"
	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/log"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/logs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// note: when adding/changing fields here also make sure to carry it over to
// update/application.go.
type applicationCmd struct {
	Name        string            `arg:"" default:"" help:"Name of the app. A random name is generated if omitted."`
	Wait        bool              `default:"true" help:"Wait until the app is fully created."`
	WaitTimeout time.Duration     `default:"15m" help:"Duration to wait for the app getting ready. Only relevant if wait is set."`
	Git         gitConfig         `embed:"" prefix:"git-"`
	Size        *string           `help:"Size of the app (defaults to \"${app_default_size}\")." placeholder:"${app_default_size}"`
	Port        *int32            `help:"Port the app is listening on (defaults to ${app_default_port})." placeholder:"${app_default_port}"`
	Replicas    *int32            `help:"Amount of replicas of the running app (defaults to ${app_default_replicas})." placeholder:"${app_default_replicas}"`
	Hosts       []string          `help:"Host names where the app can be accessed. If empty, the app will just be accessible on a generated host name on the deploio.app domain."`
	BasicAuth   *bool             `help:"Enable/Disable basic authentication for the app (defaults to ${app_default_basic_auth})." placeholder:"${app_default_basic_auth}"`
	Env         map[string]string `help:"Environment variables which are passed to the app at runtime."`
	BuildEnv    map[string]string `help:"Environment variables which are passed to the app build process."`
}

type gitConfig struct {
	URL                   string  `required:"" help:"URL to the Git repository containing the app source. Both HTTPS and SSH formats are supported."`
	SubPath               string  `help:"SubPath is a path in the git repo which contains the app code. If not given, the root directory of the git repo will be used."`
	Revision              string  `default:"main" help:"Revision defines the revision of the source to deploy the app to. This can be a commit, tag or branch."`
	Username              *string `help:"Username to use when authenticating to the git repository over HTTPS." env:"GIT_USERNAME"`
	Password              *string `help:"Password to use when authenticating to the git repository over HTTPS. In case of GitHub or GitLab, this can also be an access token." env:"GIT_PASSWORD"`
	SSHPrivateKey         *string `help:"Private key in PEM format to connect to the git repository via SSH." env:"GIT_SSH_PRIVATE_KEY" xor:"SSH_KEY"`
	SSHPrivateKeyFromFile *string `help:"Path to a file containing a private key in PEM format to connect to the git repository via SSH." env:"GIT_SSH_PRIVATE_KEY_FROM_FILE" xor:"SSH_KEY"`
}

func (g gitConfig) sshPrivateKey() (*string, error) {
	if g.SSHPrivateKey != nil {
		return validatePEM(*g.SSHPrivateKey)
	}
	if g.SSHPrivateKeyFromFile == nil {
		return nil, nil
	}
	content, err := os.ReadFile(*g.SSHPrivateKeyFromFile)
	if err != nil {
		return nil, err
	}
	return validatePEM(string(content))
}

const (
	buildStatusRunning = "running"
	buildStatusSuccess = "success"
	buildStatusError   = "error"
	buildStatusUnknown = "unknown"

	releaseStatusAvailable      = "available"
	releaseStatusReplicaFailure = "replicaFailure"
)

func (app *applicationCmd) Run(ctx context.Context, client *api.Client) error {
	fmt.Println("Creating a new application")
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

	if auth.Enabled() {
		// for git auth we create a separate secret and then reference it in the app.
		secret := auth.Secret(newApp)
		if err := client.Create(ctx, secret); err != nil {
			return fmt.Errorf("unable to create git auth secret: %w", err)
		}

		newApp.Spec.ForProvider.Git.Auth = &apps.GitAuth{
			FromSecret: &meta.LocalReference{
				Name: secret.GetName(),
			},
		}
	}

	c := newCreator(client, newApp, strings.ToLower(apps.ApplicationKind))
	appWaitCtx, cancel := context.WithTimeout(ctx, app.WaitTimeout)
	defer cancel()

	if err := c.createResource(appWaitCtx); err != nil {
		return err
	}

	if !app.Wait {
		return nil
	}

	if err := c.wait(
		appWaitCtx,
		waitForBuildStart(newApp),
		waitForBuildFinish(newApp),
		waitForRelease(newApp),
	); err != nil {
		if buildErr, ok := err.(buildError); ok {
			if err := buildErr.printMessage(appWaitCtx, client); err != nil {
				return fmt.Errorf("%s: %w", buildErr, err)
			}
		}
		if releaseErr, ok := err.(releaseError); ok {
			if err := releaseErr.printMessage(appWaitCtx, client, newApp); err != nil {
				return fmt.Errorf("%s: %w", releaseErr, err)
			}
		}
		return err
	}

	if err := client.Get(ctx, api.ObjectName(newApp), newApp); err != nil {
		return err
	}

	fmt.Printf("\nYour application %q is now available at:\n  https://%s\n\n", newApp.Name, newApp.Status.AtProvider.CNAMETarget)
	printUnverifiedHostsMessage(newApp)

	if newApp.Status.AtProvider.BasicAuthSecret == nil {
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

func (app *applicationCmd) config() apps.Config {
	config := apps.Config{
		EnableBasicAuth: app.BasicAuth,
		Env:             util.EnvVarsFromMap(app.Env),
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

func waitForBuildFinish(app *apps.Application) waitStage {
	return waitStage{
		kind:       strings.ToLower(apps.BuildKind),
		objectList: &apps.BuildList{},
		listOpts: []runtimeclient.ListOption{
			runtimeclient.InNamespace(app.GetNamespace()),
			runtimeclient.MatchingLabels{util.ApplicationNameLabel: app.GetName()},
		},
		waitMessage: &message{
			text: "building application",
			icon: "📦",
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
			case buildStatusSuccess:
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

type releaseError struct {
	release *apps.Release
}

func (r releaseError) Error() string {
	return fmt.Sprintf("release failed with status %s", r.release.Status.AtProvider.ReleaseStatus)
}

func (r releaseError) printMessage(ctx context.Context, client *api.Client, app *apps.Application) error {
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
			case releaseStatusReplicaFailure:
				return false, releaseError{release: release}
			}

			return false, nil
		},
	}
}

func printUnverifiedHostsMessage(app *apps.Application) {
	unverifiedHosts := util.UnverifiedAppHosts(app)

	if len(unverifiedHosts) != 0 {
		fmt.Println("You configured the following hosts:")

		for _, name := range unverifiedHosts {
			fmt.Printf("  %s\n", name)
		}

		fmt.Printf("\nTo make your app available on them, make sure they have a CNAME record targeting %q.\n",
			app.Status.AtProvider.CNAMETarget)
	}
}

func printBuildLogs(ctx context.Context, client *api.Client, build *apps.Build) error {
	return client.Log.QueryRange(
		ctx, client.Log.StdOut,
		errorLogQuery(logs.BuildQuery(build.Name, build.Namespace)),
	)
}

func printReleaseLogs(ctx context.Context, client *api.Client, release *apps.Release) error {
	return client.Log.QueryRange(
		ctx, client.Log.StdOut,
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

// we print the last 20 lines of the log. In most cases this should be
// enough to give a hint about the problem but we might need to tweak this
// value a bit in the future.
const errorLogLines = 20

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

// validatePEM validates if the passed content is in valid PEM format
func validatePEM(content string) (*string, error) {
	content = strings.TrimSpace(content)
	b, rest := pem.Decode([]byte(content))
	if b == nil || len(rest) > 0 {
		return nil, fmt.Errorf("no valid PEM formatted data found")
	}
	return &content, nil
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
	return result, nil
}
