package create

import (
	"bytes"
	"context"
	"testing"
	"time"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/grafana/loki/pkg/logcli/output"
	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/log"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
)

func TestApplication(t *testing.T) {
	apiClient, err := test.SetupClient()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	cases := map[string]struct {
		cmd      applicationCmd
		checkApp func(t *testing.T, cmd applicationCmd, app *apps.Application)
	}{
		"without git auth": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:      "https://github.com/ninech/doesnotexist.git",
					SubPath:  "/my/app",
					Revision: "superbug",
				},
				Wait:      false,
				Name:      "custom-name",
				Size:      "mini",
				Hosts:     []string{"custom.example.org", "custom2.example.org"},
				Port:      1337,
				Replicas:  42,
				BasicAuth: false,
				Env:       map[string]string{"hello": "world"},
				BuildEnv:  map[string]string{"BP_GO_TARGETS": "./cmd/web-server"},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				assert.Equal(t, cmd.Name, app.Name)
				assert.Equal(t, cmd.Git.URL, app.Spec.ForProvider.Git.URL)
				assert.Equal(t, cmd.Git.SubPath, app.Spec.ForProvider.Git.SubPath)
				assert.Equal(t, cmd.Git.Revision, app.Spec.ForProvider.Git.Revision)
				assert.Equal(t, cmd.Hosts, app.Spec.ForProvider.Hosts)
				assert.Equal(t, apps.ApplicationSize(cmd.Size), app.Spec.ForProvider.Config.Size)
				assert.Equal(t, int32(cmd.Port), *app.Spec.ForProvider.Config.Port)
				assert.Equal(t, int32(cmd.Replicas), *app.Spec.ForProvider.Config.Replicas)
				assert.Equal(t, cmd.BasicAuth, *app.Spec.ForProvider.Config.EnableBasicAuth)
				assert.Equal(t, util.EnvVarsFromMap(cmd.Env), app.Spec.ForProvider.Config.Env)
				assert.Equal(t, util.EnvVarsFromMap(cmd.BuildEnv), app.Spec.ForProvider.BuildEnv)
				assert.Nil(t, app.Spec.ForProvider.Git.Auth)
			},
		},
		"with basic auth": {
			cmd: applicationCmd{
				Wait:      false,
				Name:      "basic-auth",
				Size:      "mini",
				BasicAuth: true,
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				assert.Equal(t, cmd.Name, app.Name)
				assert.Equal(t, apps.ApplicationSize(cmd.Size), app.Spec.ForProvider.Config.Size)
				assert.Equal(t, cmd.BasicAuth, *app.Spec.ForProvider.Config.EnableBasicAuth)
			},
		},
		"with user/pass git auth": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:      "https://github.com/ninech/doesnotexist.git",
					Username: pointer.String("deploy"),
					Password: pointer.String("hunter2"),
				},
				Wait: false,
				Name: "user-pass-auth",
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				auth := util.GitAuth{Username: cmd.Git.Username, Password: cmd.Git.Password}
				authSecret := auth.Secret(app)
				if err := apiClient.Get(ctx, api.ObjectName(authSecret), authSecret); err != nil {
					t.Fatal(err)
				}

				assert.Equal(t, *cmd.Git.Username, string(authSecret.Data[util.UsernameSecretKey]))
				assert.Equal(t, *cmd.Git.Password, string(authSecret.Data[util.PasswordSecretKey]))
				assert.Equal(t, authSecret.Annotations[util.ManagedByAnnotation], util.NctlName)
			},
		},
		"with ssh key git auth": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:           "https://github.com/ninech/doesnotexist.git",
					SSHPrivateKey: pointer.String("fakekey"),
				},
				Wait: false,
				Name: "ssh-key-auth",
				Size: "mini",
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				auth := util.GitAuth{SSHPrivateKey: cmd.Git.SSHPrivateKey}
				authSecret := auth.Secret(app)
				if err := apiClient.Get(ctx, api.ObjectName(authSecret), authSecret); err != nil {
					t.Fatal(err)
				}

				assert.Equal(t, *cmd.Git.SSHPrivateKey, string(authSecret.Data[util.PrivateKeySecretKey]))
				assert.Equal(t, authSecret.Annotations[util.ManagedByAnnotation], util.NctlName)
			},
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			app := tc.cmd.newApplication("default")

			if err := tc.cmd.Run(ctx, apiClient); err != nil {
				t.Fatal(err)
			}

			if err := apiClient.Get(ctx, api.ObjectName(app), app); err != nil {
				t.Fatal(err)
			}

			tc.checkApp(t, tc.cmd, app)
		})
	}
}

func TestApplicationWait(t *testing.T) {
	cmd := applicationCmd{
		Wait:        true,
		WaitTimeout: time.Second * 5,
		Name:        "some-name",
		BasicAuth:   true,
	}
	project := "default"

	build := &apps.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "any-name",
			Namespace: project,
			Labels: map[string]string{
				util.ApplicationNameLabel: cmd.Name,
			},
		},
	}

	release := &apps.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "another-name",
			Namespace: project,
			Labels: map[string]string{
				util.ApplicationNameLabel: cmd.Name,
			},
		},
	}

	// we are also creating a basic auth secret
	basicAuth := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name-basic-auth",
			Namespace: project,
			Labels: map[string]string{
				util.ApplicationNameLabel: cmd.Name,
			},
		},
		Data: map[string][]byte{
			util.BasicAuthUsernameKey: []byte("some-name"),
			util.BasicAuthPasswordKey: []byte("some-password"),
		},
	}

	// throw in a second build/release to ensure it can handle it
	build2 := *build
	build2.Name = build2.Name + "-1"
	release2 := *release
	release2.Name = release2.Name + "-1"

	apiClient, err := test.SetupClient(build, &build2, release, &release2, basicAuth)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// to test the wait we create a ticker that continously updates our
	// resources in a goroutine to simulate a controller doing the same
	ticker := time.NewTicker(100 * time.Millisecond)
	done := make(chan bool)
	errors := make(chan error, 1)

	go func() {
		for {
			select {
			case <-done:
				close(errors)
				return
			case <-ticker.C:
				app := &apps.Application{}
				if err := apiClient.Get(ctx, types.NamespacedName{Name: cmd.Name, Namespace: project}, app); err != nil {
					errors <- err
				}

				if err := setResourceCondition(ctx, apiClient, app, runtimev1.ReconcileSuccess()); err != nil {
					errors <- err
				}

				app.Status.AtProvider.Hosts = []apps.VerificationStatus{{Name: "host.example.org"}}
				app.Status.AtProvider.CNAMETarget = "some.target.example.org"
				app.Status.AtProvider.BasicAuthSecret = &meta.LocalReference{Name: basicAuth.Name}
				if err := apiClient.Status().Update(ctx, app); err != nil {
					errors <- err
				}

				if err := setResourceCondition(ctx, apiClient, build, runtimev1.Available()); err != nil {
					errors <- err
				}

				if err := setResourceCondition(ctx, apiClient, &build2, runtimev1.Available()); err != nil {
					errors <- err
				}

				if err := apiClient.Get(ctx, api.ObjectName(build), build); err != nil {
					errors <- err
				}

				build.Status.AtProvider.BuildStatus = buildStatusRunning
				if err := apiClient.Status().Update(ctx, build); err != nil {
					errors <- err
				}

				build.Status.AtProvider.BuildStatus = buildStatusSuccess
				if err := apiClient.Status().Update(ctx, build); err != nil {
					errors <- err
				}

				if err := setResourceCondition(ctx, apiClient, &release2, runtimev1.Available()); err != nil {
					errors <- err
				}

				release.Status.AtProvider.ReleaseStatus = releaseStatusAvailable
				if err := apiClient.Status().Update(ctx, release); err != nil {
					errors <- err
				}
			}
		}
	}()

	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}

	ticker.Stop()
	done <- true

	for err := range errors {
		t.Fatal(err)
	}
}

func TestApplicationBuildFail(t *testing.T) {
	cmd := applicationCmd{
		Wait:        true,
		WaitTimeout: time.Second * 5,
		Name:        "some-name",
	}
	project := "default"

	build := &apps.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "any-name",
			Namespace: project,
			Labels: map[string]string{
				util.ApplicationNameLabel: cmd.Name,
			},
		},
	}

	client, err := test.SetupClient(build)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	out, err := output.NewLogOutput(&buf, log.Mode("default"), &output.LogOutputOptions{
		NoLabels: true, ColoredOutput: false, Timezone: time.Local,
	})
	if err != nil {
		t.Fatal(err)
	}

	// fill our logs so we have more than errorLogLines
	logString := "does not compute!"
	buildLog := []string{}
	for i := 0; i < errorLogLines+30; i++ {
		buildLog = append(buildLog, logString)
	}
	client.Log = &log.Client{Client: log.NewFake(t, time.Now(), buildLog...), StdOut: out}

	ctx := context.Background()

	// to test the wait we create a ticker that continously updates our
	// resources in a goroutine to simulate a controller doing the same
	ticker := time.NewTicker(100 * time.Millisecond)
	done := make(chan bool)
	errors := make(chan error, 1)

	go func() {
		for {
			select {
			case <-done:
				close(errors)
				return
			case <-ticker.C:
				app := &apps.Application{ObjectMeta: metav1.ObjectMeta{
					Name:      cmd.Name,
					Namespace: project,
				}}

				if err := setResourceCondition(ctx, client, app, runtimev1.ReconcileSuccess()); err != nil {
					errors <- err
				}

				build.Status.AtProvider.BuildStatus = buildStatusError
				if err := client.Status().Update(ctx, build); err != nil {
					errors <- err
				}
			}
		}
	}()

	if err := cmd.Run(ctx, client); err == nil {
		t.Fatal("expected build error, got nil")
	}

	ticker.Stop()
	done <- true

	for err := range errors {
		t.Fatal(err)
	}

	assert.Contains(t, buf.String(), logString)
	assert.Equal(t, test.CountLines(buf.String()), errorLogLines)
}

func setResourceCondition(ctx context.Context, apiClient *api.Client, mg resource.Managed, condition runtimev1.Condition) error {
	if err := apiClient.Get(ctx, api.ObjectName(mg), mg); err != nil {
		return err
	}

	mg.SetConditions(condition)
	return apiClient.Status().Update(ctx, mg)
}
