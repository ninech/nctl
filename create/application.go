package create

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/grafana/loki/pkg/logproto"
	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/log"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/logs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/utils/pointer"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// note: when adding/changing fields here also make sure to carry it over to
// update/application.go.
type applicationCmd struct {
	Name        string            `arg:"" default:"" help:"Name of the application. A random name is generated if omitted."`
	Wait        bool              `default:"true" help:"Wait until application is fully created."`
	WaitTimeout time.Duration     `default:"10m" help:"Duration to wait for application getting ready. Only relevant if wait is set."`
	Git         gitConfig         `embed:"" prefix:"git-"`
	Size        string            `default:"micro" help:"Size of the app."`
	Port        int32             `default:"8080" help:"Port the app is listening on."`
	Replicas    int32             `default:"1" help:"Amount of replicas of the running app."`
	Hosts       []string          `help:"Host names where the application can be accessed. If empty, the application will just be accessible on a generated host name on the deploio.app domain."`
	Env         map[string]string `help:"Environment variables which are passed to the app at runtime."`
}

type gitConfig struct {
	URL           string  `required:"" help:"URL to the Git repository containing the application source. Both HTTPS and SSH formats are supported."`
	SubPath       string  `help:"SubPath is a path in the git repo which contains the application code. If not given, the root directory of the git repo will be used."`
	Revision      string  `default:"main" help:"Revision defines the revision of the source to deploy the application to. This can be a commit, tag or branch."`
	Username      *string `help:"Username to use when authenticating to the git repository over HTTPS." env:"GIT_USERNAME"`
	Password      *string `help:"Password to use when authenticating to the git repository over HTTPS. In case of GitHub or GitLab, this can also be an access token." env:"GIT_PASSWORD"`
	SSHPrivateKey *string `help:"Private key in x509 format to connect to the git repository via SSH." env:"GIT_SSH_PRIVATE_KEY"`
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

	auth := util.GitAuth{
		Username:      app.Git.Username,
		Password:      app.Git.Password,
		SSHPrivateKey: app.Git.SSHPrivateKey,
	}

	if auth.Enabled() {
		// for git auth we create a separate secret and then reference it in the app.
		secret := auth.Secret(newApp.Name, client.Project)
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
	ctx, cancel := context.WithTimeout(ctx, app.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !app.Wait {
		return nil
	}

	if err := c.wait(
		ctx,
		waitForBuildStart(newApp),
		waitForBuildFinish(newApp),
		waitForRelease(newApp),
	); err != nil {
		if buildErr, ok := err.(buildError); ok {
			if err := buildErr.printMessage(ctx, client); err != nil {
				return fmt.Errorf("%s: %w", buildErr, err)
			}
		}
		if releaseErr, ok := err.(releaseError); ok {
			if err := releaseErr.printMessage(ctx, client, newApp); err != nil {
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
	return nil
}

func (app *applicationCmd) newApplication(project string) *apps.Application {
	name := getName(app.Name)
	size := apps.ApplicationSize(app.Size)

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
				Hosts: app.Hosts,
				Config: apps.Config{
					Size:     &size,
					Replicas: pointer.Int32(app.Replicas),
					Port:     pointer.Int32(app.Port),
					Env:      util.EnvVarsFromMap(app.Env),
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
