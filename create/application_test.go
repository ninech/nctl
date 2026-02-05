package create

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/grafana/loki/v3/pkg/logcli/output"
	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/log"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

func createTempKeyFile(content string) (string, error) {
	file, err := os.CreateTemp("", "temp-private-ssh-key.*.pem")
	if err != nil {
		return "", err
	}
	_, err = file.WriteString(content)
	if err != nil {
		return "", err
	}
	return file.Name(), nil
}

func TestApplication(t *testing.T) {
	apiClient, err := test.SetupClient()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	dummyRSAKey, err := test.GenerateRSAPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	filenameRSAKey, err := createTempKeyFile("   " + dummyRSAKey + "     ")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(filenameRSAKey)

	dummyED25519Key, err := test.GenerateED25519PrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	filenameED25519Key, err := createTempKeyFile(dummyED25519Key)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(filenameED25519Key)

	gitInfoService := test.NewGitInformationService()
	gitInfoService.Start()
	defer gitInfoService.Close()

	cases := map[string]struct {
		cmd                           applicationCmd
		checkApp                      func(t *testing.T, cmd applicationCmd, app *apps.Application)
		gitInformationServiceResponse test.GitInformationServiceResponse
		errorExpected                 bool
	}{
		"without git auth": {
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Wait: false,
					Name: "custom-name",
				},
				Git: gitConfig{
					URL:      "https://github.com/ninech/doesnotexist.git",
					SubPath:  "/my/app",
					Revision: "superbug",
				},
				Size:                ptr.To("mini"),
				Hosts:               []string{"custom.example.org", "custom2.example.org"},
				Port:                ptr.To(int32(1337)),
				HealthProbe:         healthProbe{PeriodSeconds: int32(7), Path: "/he"},
				Replicas:            ptr.To(int32(42)),
				BasicAuth:           ptr.To(false),
				Env:                 map[string]string{"hello": "world"},
				BuildEnv:            map[string]string{"BP_GO_TARGETS": "./cmd/web-server"},
				DeployJob:           deployJob{Command: "date", Name: "print-date", Retries: 2, Timeout: time.Minute},
				SkipRepoAccessCheck: true,
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				assert.Equal(t, cmd.Name, app.Name)
				assert.Equal(t, cmd.Git.URL, app.Spec.ForProvider.Git.URL)
				assert.Equal(t, cmd.Git.SubPath, app.Spec.ForProvider.Git.SubPath)
				assert.Equal(t, cmd.Git.Revision, app.Spec.ForProvider.Git.Revision)
				assert.Equal(t, cmd.Hosts, app.Spec.ForProvider.Hosts)
				assert.Equal(t, apps.ApplicationSize(*cmd.Size), app.Spec.ForProvider.Config.Size)
				assert.Equal(t, *cmd.Port, *app.Spec.ForProvider.Config.Port)
				assert.Equal(t, cmd.HealthProbe.PeriodSeconds, *app.Spec.ForProvider.Config.HealthProbe.PeriodSeconds)
				assert.Equal(t, cmd.HealthProbe.Path, app.Spec.ForProvider.Config.HealthProbe.HTTPGet.Path)
				assert.Equal(t, *cmd.Replicas, *app.Spec.ForProvider.Config.Replicas)
				assert.Equal(t, *cmd.BasicAuth, *app.Spec.ForProvider.Config.EnableBasicAuth)
				assert.Equal(t, util.EnvVarsFromMap(cmd.Env), app.Spec.ForProvider.Config.Env)
				assert.Equal(t, util.EnvVarsFromMap(cmd.BuildEnv), app.Spec.ForProvider.BuildEnv)
				assert.Equal(t, cmd.DeployJob.Command, app.Spec.ForProvider.Config.DeployJob.Command)
				assert.Equal(t, cmd.DeployJob.Name, app.Spec.ForProvider.Config.DeployJob.Name)
				assert.Equal(t, cmd.DeployJob.Timeout, app.Spec.ForProvider.Config.DeployJob.Timeout.Duration)
				assert.Equal(t, cmd.DeployJob.Retries, *app.Spec.ForProvider.Config.DeployJob.Retries)
				assert.Nil(t, app.Spec.ForProvider.Git.Auth)
			},
		},
		"with basic auth": {
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Wait: false,
					Name: "basic-auth",
				},
				Size:                ptr.To("mini"),
				BasicAuth:           ptr.To(true),
				SkipRepoAccessCheck: true,
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				assert.Equal(t, cmd.Name, app.Name)
				assert.Equal(t, apps.ApplicationSize(*cmd.Size), app.Spec.ForProvider.Config.Size)
				assert.Equal(t, *cmd.BasicAuth, *app.Spec.ForProvider.Config.EnableBasicAuth)
			},
		},
		"with user/pass git auth": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:      "https://github.com/ninech/doesnotexist.git",
					Username: ptr.To("deploy"),
					Password: ptr.To("hunter2"),
				},
				resourceCmd: resourceCmd{
					Wait: false,
					Name: "user-pass-auth",
				},
				SkipRepoAccessCheck: true,
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
					SSHPrivateKey: &dummyRSAKey,
				},
				resourceCmd: resourceCmd{
					Wait: false,
					Name: "ssh-key-auth",
				},
				Size:                ptr.To("mini"),
				SkipRepoAccessCheck: true,
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				auth := util.GitAuth{SSHPrivateKey: cmd.Git.SSHPrivateKey}
				authSecret := auth.Secret(app)
				if err := apiClient.Get(ctx, api.ObjectName(authSecret), authSecret); err != nil {
					t.Fatal(err)
				}

				assert.Equal(t, strings.TrimSpace(*cmd.Git.SSHPrivateKey), string(authSecret.Data[util.PrivateKeySecretKey]))
				assert.Equal(t, authSecret.Annotations[util.ManagedByAnnotation], util.NctlName)
			},
		},
		"with ssh ed25519 key git auth": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:           "https://github.com/ninech/doesnotexist.git",
					SSHPrivateKey: &dummyED25519Key,
				},
				resourceCmd: resourceCmd{
					Wait: false,
					Name: "ssh-key-auth-ed25519",
				},
				Size:                ptr.To("mini"),
				SkipRepoAccessCheck: true,
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				auth := util.GitAuth{SSHPrivateKey: cmd.Git.SSHPrivateKey}
				authSecret := auth.Secret(app)
				if err := apiClient.Get(ctx, api.ObjectName(authSecret), authSecret); err != nil {
					t.Fatal(err)
				}

				assert.Equal(t, strings.TrimSpace(*cmd.Git.SSHPrivateKey), string(authSecret.Data[util.PrivateKeySecretKey]))
				assert.Equal(t, authSecret.Annotations[util.ManagedByAnnotation], util.NctlName)
			},
		},
		"with ssh key git auth from file": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:                   "https://github.com/ninech/doesnotexist.git",
					SSHPrivateKeyFromFile: ptr.To(filenameRSAKey),
				},
				resourceCmd: resourceCmd{
					Wait: false,
					Name: "ssh-key-auth-from-file",
				},
				Size:                ptr.To("mini"),
				SkipRepoAccessCheck: true,
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				auth := util.GitAuth{SSHPrivateKey: ptr.To("notused")}
				authSecret := auth.Secret(app)
				if err := apiClient.Get(ctx, api.ObjectName(authSecret), authSecret); err != nil {
					t.Fatal(err)
				}

				assert.Equal(t, dummyRSAKey, string(authSecret.Data[util.PrivateKeySecretKey]))
				assert.Equal(t, authSecret.Annotations[util.ManagedByAnnotation], util.NctlName)
			},
		},
		"with ed25519 ssh key git auth from file": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:                   "https://github.com/ninech/doesnotexist.git",
					SSHPrivateKeyFromFile: ptr.To(filenameED25519Key),
				},
				resourceCmd: resourceCmd{
					Wait: false,
					Name: "ssh-key-auth-from-file-ed25519",
				},
				Size:                ptr.To("mini"),
				SkipRepoAccessCheck: true,
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				auth := util.GitAuth{SSHPrivateKey: ptr.To("notused")}
				authSecret := auth.Secret(app)
				if err := apiClient.Get(ctx, api.ObjectName(authSecret), authSecret); err != nil {
					t.Fatal(err)
				}

				assert.Equal(t, strings.TrimSpace(dummyED25519Key), string(authSecret.Data[util.PrivateKeySecretKey]))
				assert.Equal(t, authSecret.Annotations[util.ManagedByAnnotation], util.NctlName)
			},
		},
		"with non valid ssh key": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:           "https://github.com/ninech/doesnotexist.git",
					SSHPrivateKey: ptr.To("not valid"),
				},
				resourceCmd: resourceCmd{
					Wait: false,
					Name: "ssh-key-auth-non-valid",
				},
				Size:                ptr.To("mini"),
				SkipRepoAccessCheck: true,
			},
			errorExpected: true,
		},
		"deploy job empty command": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL: "https://github.com/ninech/doesnotexist.git",
				},
				resourceCmd: resourceCmd{
					Wait: false,
					Name: "deploy-job-empty-command",
				},
				Size:                ptr.To("mini"),
				DeployJob:           deployJob{Command: "", Name: "print-date", Retries: 2, Timeout: time.Minute},
				SkipRepoAccessCheck: true,
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				assert.Nil(t, app.Spec.ForProvider.Config.DeployJob)
			},
		},
		"deploy job empty name": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL: "https://github.com/ninech/doesnotexist.git",
				},
				resourceCmd: resourceCmd{
					Wait: false,
					Name: "deploy-job-empty-name",
				},
				Size:                ptr.To("mini"),
				DeployJob:           deployJob{Command: "date", Name: "", Retries: 2, Timeout: time.Minute},
				SkipRepoAccessCheck: true,
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				assert.Nil(t, app.Spec.ForProvider.Config.DeployJob)
			},
		},
		"git-information-service happy path": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:      "https://github.com/ninech/doesnotexist.git",
					SubPath:  "/my/app",
					Revision: "superbug",
				},
				resourceCmd: resourceCmd{
					Wait: false,
					Name: "git-information-happy-path",
				},
				Size: ptr.To("mini"),
			},
			gitInformationServiceResponse: test.GitInformationServiceResponse{
				Code: 200,
				Content: apps.GitExploreResponse{
					RepositoryInfo: &apps.RepositoryInfo{
						URL:      "https://github.com/ninech/doesnotexist.git",
						Branches: []string{"main"},
						Tags:     []string{"superbug"},
						RevisionResponse: &apps.RevisionResponse{
							RevisionRequested: "superbug",
							Found:             true,
						},
					},
				},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				assert.Equal(t, cmd.Name, app.Name)
				assert.Equal(t, cmd.Git.URL, app.Spec.ForProvider.Git.URL)
				assert.Equal(t, cmd.Git.SubPath, app.Spec.ForProvider.Git.SubPath)
				assert.Equal(t, cmd.Git.Revision, app.Spec.ForProvider.Git.Revision)
				assert.Equal(t, apps.ApplicationSize(*cmd.Size), app.Spec.ForProvider.Config.Size)
				assert.Nil(t, app.Spec.ForProvider.Git.Auth)
			},
		},
		"git-information-service received errors": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:      "https://github.com/ninech/doesnotexist.git",
					SubPath:  "/my/app",
					Revision: "superbug",
				},
				resourceCmd: resourceCmd{
					Wait: false,
					Name: "git-information-errors",
				},
				Size: ptr.To("mini"),
			},
			gitInformationServiceResponse: test.GitInformationServiceResponse{
				Code: 200,
				Content: apps.GitExploreResponse{
					Error: "repository does not exist",
				},
			},
			errorExpected: true,
		},
		"git-information-service revision unknown": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:      "https://github.com/ninech/doesnotexist.git",
					SubPath:  "/my/app",
					Revision: "notexistent",
				},
				resourceCmd: resourceCmd{
					Wait: false,
					Name: "git-information-unknown-revision",
				},
				Size: ptr.To("mini"),
			},
			gitInformationServiceResponse: test.GitInformationServiceResponse{
				Code: 200,
				Content: apps.GitExploreResponse{
					RepositoryInfo: &apps.RepositoryInfo{
						URL:      "https://github.com/ninech/doesnotexist.git",
						Branches: []string{"main"},
						Tags:     []string{"v1.0"},
						RevisionResponse: &apps.RevisionResponse{
							RevisionRequested: "notexistent",
							Found:             false,
						},
					},
				},
			},
			errorExpected: true,
		},
		"git-information-service has issues": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:      "https://github.com/ninech/doesnotexist.git",
					SubPath:  "/my/app",
					Revision: "notexistent",
				},
				resourceCmd: resourceCmd{
					Wait: false,
					Name: "git-information-unknown-revision",
				},
				Size: ptr.To("mini"),
			},
			gitInformationServiceResponse: test.GitInformationServiceResponse{
				Code: 501,
				Raw:  ptr.To("maintenance mode - we will be back soon"),
			},
			errorExpected: true,
		},
		"git URL without proper scheme should be updated to HTTPS on success": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:      "github.com/ninech/doesnotexist.git",
					SubPath:  "/my/app",
					Revision: "main",
				},
				resourceCmd: resourceCmd{
					Wait: false,
					Name: "git-information-update-url-to-https",
				},
				Size: ptr.To("mini"),
			},
			gitInformationServiceResponse: test.GitInformationServiceResponse{
				Code: 200,
				Content: apps.GitExploreResponse{
					RepositoryInfo: &apps.RepositoryInfo{
						URL:      "https://github.com/ninech/doesnotexist.git",
						Branches: []string{"main"},
						RevisionResponse: &apps.RevisionResponse{
							RevisionRequested: "main",
							Found:             true,
						},
					},
				},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				assert.Equal(t, cmd.Name, app.Name)
				assert.Equal(t, "https://github.com/ninech/doesnotexist.git", app.Spec.ForProvider.Git.URL)
				assert.Equal(t, cmd.Git.SubPath, app.Spec.ForProvider.Git.SubPath)
				assert.Equal(t, cmd.Git.Revision, app.Spec.ForProvider.Git.Revision)
				assert.Equal(t, apps.ApplicationSize(*cmd.Size), app.Spec.ForProvider.Config.Size)
				assert.Nil(t, app.Spec.ForProvider.Git.Auth)
			},
		},
		"with sensitive env": {
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: "sensitive-env-test",
				},
				Git: gitConfig{
					URL:      "https://github.com/ninech/doesnotexist.git",
					SubPath:  "/my/app",
					Revision: "superbug",
				},
				SensitiveEnv:        map[string]string{"secret": "orange"},
				SensitiveBuildEnv:   map[string]string{"build_secret": "banana"},
				SkipRepoAccessCheck: true,
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				env := util.EnvVarByName(app.Spec.ForProvider.Config.Env, "secret")
				require.NotNil(t, env)
				require.NotNil(t, env.Sensitive)
				assert.True(t, *env.Sensitive)
				assert.Equal(t, "orange", env.Value)

				buildEnv := util.EnvVarByName(app.Spec.ForProvider.BuildEnv, "build_secret")
				require.NotNil(t, buildEnv)
				require.NotNil(t, buildEnv.Sensitive)
				assert.True(t, *buildEnv.Sensitive)
				assert.Equal(t, "banana", buildEnv.Value)
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if tc.cmd.GitInformationServiceURL == "" {
				tc.cmd.GitInformationServiceURL = gitInfoService.URL()
			}
			gitInfoService.SetResponse(tc.gitInformationServiceResponse)
			app := tc.cmd.newApplication("default")

			err := tc.cmd.Run(ctx, apiClient)
			if tc.errorExpected {
				require.Error(t, err)
				return
			} else {
				require.NoError(t, err)
			}

			require.NoError(t, apiClient.Get(ctx, api.ObjectName(app), app))

			tc.checkApp(t, tc.cmd, app)
		})
	}
}

func TestApplicationWait(t *testing.T) {
	cmd := applicationCmd{
		resourceCmd: resourceCmd{
			Wait:        true,
			WaitTimeout: time.Second * 5,
			Name:        "some-name",
		},
		BasicAuth:           ptr.To(true),
		SkipRepoAccessCheck: true,
	}
	project := test.DefaultProject

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
		Spec: apps.ReleaseSpec{
			ForProvider: apps.ReleaseParameters{
				Configuration: apps.Config{
					EnableBasicAuth: ptr.To(true),
				}.WithOrigin(apps.ConfigOriginApplication),
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

	apiClient, err := test.SetupClient(test.WithObjects(build, &build2, release, &release2, basicAuth))
	if err != nil {
		t.Fatal(err)
	}

	out, err := log.StdOut("default")
	if err != nil {
		t.Fatal(err)
	}

	apiClient.Log = &log.Client{Client: log.NewFake(t, time.Now(), "one", "two"), StdOut: out}

	ctx := context.Background()

	// to test the wait we create a ticker that continuously updates our
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

				app.Status.AtProvider.Hosts = meta.DNSVerificationStatusEntries{{Name: "host.example.org"}}
				app.Status.AtProvider.CNAMETarget = "some.target.example.org"
				app.Status.AtProvider.BasicAuthSecret = &meta.LocalReference{Name: basicAuth.Name}
				app.Status.AtProvider.LatestRelease = release2.Name
				if err := apiClient.Update(ctx, app); err != nil {
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
				if err := apiClient.Update(ctx, build); err != nil {
					errors <- err
				}

				build.Status.AtProvider.BuildStatus = buildStatusSuccess
				if err := apiClient.Update(ctx, build); err != nil {
					errors <- err
				}

				if err := setResourceCondition(ctx, apiClient, &release2, runtimev1.Available()); err != nil {
					errors <- err
				}

				release.Status.AtProvider.ReleaseStatus = releaseStatusAvailable
				if err := apiClient.Update(ctx, release); err != nil {
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
		resourceCmd: resourceCmd{
			Wait:        true,
			WaitTimeout: time.Second * 5,
			Name:        "some-name",
		},
		SkipRepoAccessCheck: true,
	}
	project := test.DefaultProject

	build := &apps.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "any-name",
			Namespace: project,
			Labels: map[string]string{
				util.ApplicationNameLabel: cmd.Name,
			},
		},
	}

	client, err := test.SetupClient(test.WithObjects(build))
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
	for range errorLogLines + 30 {
		buildLog = append(buildLog, logString)
	}
	client.Log = &log.Client{Client: log.NewFake(t, time.Now(), buildLog...), StdOut: out}

	ctx := context.Background()

	// to test the wait we create a ticker that continuously updates our
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
				if err := client.Update(ctx, build); err != nil {
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
	return apiClient.Update(ctx, mg)
}
