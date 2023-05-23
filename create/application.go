package create

import (
	"context"
	"fmt"
	"strings"
	"time"

	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/utils/pointer"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

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
	URL           string `required:"" help:"URL to the Git repository containing the application source. Both HTTPS and SSH formats are supported."`
	SubPath       string `help:"SubPath is a path in the git repo which contains the application code. If not given, the root directory of the git repo will be used."`
	Revision      string `default:"main" help:"Revision defines the revision of the source to deploy the application to. This can be a commit, tag or branch."`
	Username      string `help:"Username to use when authenticating to the git repository over HTTPS." env:"GIT_USERNAME"`
	Password      string `help:"Password to use when authenticating to the git repository over HTTPS. In case of GitHub or GitLab, this can also be an access token." env:"GIT_PASSWORD"`
	SSHPrivateKey string `help:"Private key in x509 format to connect to the git repository via SSH." env:"GIT_SSH_PRIVATE_KEY"`
}

const (
	applicationNameLabel = "application.apps.nine.ch/name"

	buildStatusRunning = "running"
	buildStatusSuccess = "success"
	buildStatusError   = "error"
	buildStatusUnknown = "unknown"

	releaseStatusAvailable      = "available"
	releaseStatusReplicaFailure = "replicaFailure"
)

func (app *applicationCmd) Run(ctx context.Context, client *api.Client) error {
	fmt.Println("Creating a new application")
	newApp := app.newApplication(client.Namespace)

	if gitAuthEnabled(app.Git) {
		// for git auth we create a separate secret and then reference it in the app.
		secret := gitAuthSecret(app.Git, newApp.Name, client.Namespace)
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
			buildErr.printMessage()
		}
		if releaseErr, ok := err.(releaseError); ok {
			releaseErr.printMessage()
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

func (app *applicationCmd) newApplication(namespace string) *apps.Application {
	name := getName(app.Name)
	size := apps.ApplicationSize(app.Size)

	return &apps.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
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
					Env:      toEnvVars(app.Env),
				},
			},
		},
	}
}

func gitAuthEnabled(git gitConfig) bool {
	if len(git.Username) != 0 ||
		len(git.Password) != 0 ||
		len(git.SSHPrivateKey) != 0 {
		return true
	}

	return false
}

func gitAuthSecret(git gitConfig, name, namespace string) *corev1.Secret {
	data := map[string][]byte{}

	if len(git.SSHPrivateKey) != 0 {
		data["privatekey"] = []byte(git.SSHPrivateKey)
	} else if len(git.Username) != 0 && len(git.Password) != 0 {
		data["username"] = []byte(git.Username)
		data["password"] = []byte(git.Password)
	}

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: data,
	}
}

func toEnvVars(env map[string]string) apps.EnvVars {
	vars := apps.EnvVars{}
	for k, v := range env {
		vars = append(vars, apps.EnvVar{Name: k, Value: v})
	}
	return vars
}

type buildError struct {
	build *apps.Build
}

func (b buildError) Error() string {
	return fmt.Sprintf("build failed with status %s", b.build.Status.AtProvider.BuildStatus)
}

func (b buildError) printMessage() {
	fmt.Printf("\nYour build has failed with status %q. To view the build logs run the command:\n",
		b.build.Status.AtProvider.BuildStatus)
	fmt.Printf("  nctl logs build %s\n\n", b.build.Name)
}

func waitForBuildStart(app *apps.Application) waitStage {
	return waitStage{
		kind:       strings.ToLower(apps.BuildKind),
		objectList: &apps.BuildList{},
		listOpts: []runtimeclient.ListOption{
			runtimeclient.InNamespace(app.GetNamespace()),
			runtimeclient.MatchingLabels{applicationNameLabel: app.GetName()},
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
			runtimeclient.MatchingLabels{applicationNameLabel: app.GetName()},
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

func (r releaseError) printMessage() {
	fmt.Printf("\nYour release has failed with status %q. To view the release logs run the command:\n",
		r.release.Status.AtProvider.ReleaseStatus)
	fmt.Printf("  nctl logs release %s\n\n", r.release.Name)
}

func waitForRelease(app *apps.Application) waitStage {
	return waitStage{
		kind:       strings.ToLower(apps.ReleaseKind),
		objectList: &apps.ReleaseList{},
		listOpts: []runtimeclient.ListOption{
			runtimeclient.InNamespace(app.GetNamespace()),
			runtimeclient.MatchingLabels{applicationNameLabel: app.GetName()},
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
