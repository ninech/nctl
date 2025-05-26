package update

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/create"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestApplication(t *testing.T) {
	ctx := context.Background()
	initialSize := apps.ApplicationSize("micro")

	dummyRSAKey, err := test.GenerateRSAPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	gitInfoService := test.NewGitInformationService()
	gitInfoService.Start()
	defer gitInfoService.Close()

	existingApp := &apps.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name",
			Namespace: test.DefaultProject,
		},
		Spec: apps.ApplicationSpec{
			ForProvider: apps.ApplicationParameters{
				Git: apps.ApplicationGitConfig{
					GitTarget: apps.GitTarget{
						URL:      "https://git.example.org",
						SubPath:  "path/to/app",
						Revision: "main",
					},
				},
				Hosts: []string{"one.example.org"},
				Config: apps.Config{
					Size:            initialSize,
					Replicas:        ptr.To(int32(1)),
					Port:            ptr.To(int32(1337)),
					Env:             util.EnvVarsFromMap(map[string]string{"foo": "bar"}),
					EnableBasicAuth: ptr.To(false),
				},
				BuildEnv: util.EnvVarsFromMap(map[string]string{"BP_ENVIRONMENT_VARIABLE": "some-value"}),
			},
		},
	}

	cases := map[string]struct {
		orig                          *apps.Application
		gitAuth                       *util.GitAuth
		cmd                           applicationCmd
		checkApp                      func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application)
		checkSecret                   func(t *testing.T, cmd applicationCmd, authSecret *corev1.Secret)
		gitInformationServiceResponse test.GitInformationServiceResponse
		errorExpected                 bool
	}{
		"change port": {
			orig: existingApp,
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				baseConfig: baseConfig{
					Port: ptr.To(int32(1234)),
				},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
			},
		},
		"port is unchanged when updating unrelated field": {
			orig: existingApp,
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				baseConfig: baseConfig{
					Size: ptr.To("newsize"),
				},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
			},
		},
		"all field updates": {
			orig: existingApp,
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				Git: &gitConfig{
					URL:      ptr.To("https://newgit.example.org"),
					SubPath:  ptr.To("new/path"),
					Revision: ptr.To("some-change"),
				},
				baseConfig:          newBaseConfigCmdAllFields(),
				Hosts:               &[]string{"one.example.org", "two.example.org"},
				BuildEnv:            map[string]string{"BP_GO_TARGETS": "./cmd/web-server"},
				SkipRepoAccessCheck: true,
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
				assert.Equal(t, *cmd.Git.URL, updated.Spec.ForProvider.Git.URL)
				assert.Equal(t, *cmd.Git.SubPath, updated.Spec.ForProvider.Git.SubPath)
				assert.Equal(t, *cmd.Git.Revision, updated.Spec.ForProvider.Git.Revision)
				assert.Equal(t, *cmd.Hosts, updated.Spec.ForProvider.Hosts)
				assert.Equal(t, util.UpdateEnvVars(existingApp.Spec.ForProvider.BuildEnv, cmd.BuildEnv, nil), updated.Spec.ForProvider.BuildEnv)
				// Retry Release/Build should be not set by default:
				assert.Nil(t, util.EnvVarByName(updated.Spec.ForProvider.Config.Env, ReleaseTrigger))
				assert.Nil(t, util.EnvVarByName(updated.Spec.ForProvider.BuildEnv, BuildTrigger))
			},
		},
		"reset env variable": {
			orig: existingApp,
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				baseConfig: baseConfig{
					DeleteEnv: &[]string{"foo"},
				},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
				assert.NotEmpty(t, updated.Spec.ForProvider.BuildEnv)
			},
		},
		"change multiple env variables at once": {
			orig: existingApp,
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				baseConfig: baseConfig{
					Env: map[string]string{"bar1": "zoo", "bar2": "foo"},
				},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
			},
		},
		"reset build env variable": {
			orig: existingApp,
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				DeleteBuildEnv: &[]string{"BP_ENVIRONMENT_VARIABLE"},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
				assert.Empty(t, updated.Spec.ForProvider.BuildEnv)
			},
		},
		"change basic auth password": {
			orig: existingApp,
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				ChangeBasicAuthPassword: ptr.To(true),
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
				assert.NotNil(t, updated.Spec.ForProvider.BasicAuthPasswordChange)
			},
		},
		"git auth update user/pass": {
			orig: existingApp,
			gitAuth: &util.GitAuth{
				Username: ptr.To("some-user"),
				Password: ptr.To("some-password"),
			},
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				Git: &gitConfig{
					Username: ptr.To("new-user"),
					Password: ptr.To("new-pass"),
				},
			},
			gitInformationServiceResponse: test.GitInformationServiceResponse{
				Code: 200,
				Content: apps.GitExploreResponse{
					RepositoryInfo: &apps.RepositoryInfo{
						URL:      existingApp.Spec.ForProvider.Git.URL,
						Branches: []string{existingApp.Spec.ForProvider.Git.Revision},
						RevisionResponse: &apps.RevisionResponse{
							RevisionRequested: existingApp.Spec.ForProvider.Git.Revision,
							Found:             true,
						},
					},
				},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
			},
			checkSecret: func(t *testing.T, cmd applicationCmd, authSecret *corev1.Secret) {
				assert.Equal(t, *cmd.Git.Username, string(authSecret.Data[util.UsernameSecretKey]))
				assert.Equal(t, *cmd.Git.Password, string(authSecret.Data[util.PasswordSecretKey]))
				assert.Equal(t, authSecret.Annotations[util.ManagedByAnnotation], util.NctlName)
			},
		},
		"git auth update ssh key": {
			orig: existingApp,
			gitAuth: &util.GitAuth{
				SSHPrivateKey: ptr.To("fakekey"),
			},
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				Git: &gitConfig{
					SSHPrivateKey: &dummyRSAKey,
				},
			},
			gitInformationServiceResponse: test.GitInformationServiceResponse{
				Code: 200,
				Content: apps.GitExploreResponse{
					RepositoryInfo: &apps.RepositoryInfo{
						URL:      existingApp.Spec.ForProvider.Git.URL,
						Branches: []string{existingApp.Spec.ForProvider.Git.Revision},
						RevisionResponse: &apps.RevisionResponse{
							RevisionRequested: existingApp.Spec.ForProvider.Git.Revision,
							Found:             true,
						},
					},
				},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
			},
			checkSecret: func(t *testing.T, cmd applicationCmd, authSecret *corev1.Secret) {
				assert.Equal(t, strings.TrimSpace(*cmd.Git.SSHPrivateKey), string(authSecret.Data[util.PrivateKeySecretKey]))
				assert.Equal(t, authSecret.Annotations[util.ManagedByAnnotation], util.NctlName)
			},
		},
		"git auth update creates a secret": {
			orig:    existingApp,
			gitAuth: nil,
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				Git: &gitConfig{
					Username: ptr.To("new-user"),
					Password: ptr.To("new-pass"),
				},
			},
			gitInformationServiceResponse: test.GitInformationServiceResponse{
				Code: 200,
				Content: apps.GitExploreResponse{
					RepositoryInfo: &apps.RepositoryInfo{
						URL:      existingApp.Spec.ForProvider.Git.URL,
						Branches: []string{existingApp.Spec.ForProvider.Git.Revision},
						RevisionResponse: &apps.RevisionResponse{
							RevisionRequested: existingApp.Spec.ForProvider.Git.Revision,
							Found:             true,
						},
					},
				},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
			},
			checkSecret: func(t *testing.T, cmd applicationCmd, authSecret *corev1.Secret) {
				assert.Equal(t, *cmd.Git.Username, string(authSecret.Data[util.UsernameSecretKey]))
				assert.Equal(t, *cmd.Git.Password, string(authSecret.Data[util.PasswordSecretKey]))
				assert.Equal(t, authSecret.Annotations[util.ManagedByAnnotation], util.NctlName)
			},
		},
		"git auth is unchanged on normal field update": {
			orig: existingApp,
			gitAuth: &util.GitAuth{
				SSHPrivateKey: ptr.To("fakekey"),
			},
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				Git: &gitConfig{
					URL: ptr.To("https://newgit.example.org"),
				},
			},
			gitInformationServiceResponse: test.GitInformationServiceResponse{
				Code: 200,
				Content: apps.GitExploreResponse{
					RepositoryInfo: &apps.RepositoryInfo{
						URL:      "https://newgit.example.org",
						Branches: []string{existingApp.Spec.ForProvider.Git.Revision},
						RevisionResponse: &apps.RevisionResponse{
							RevisionRequested: existingApp.Spec.ForProvider.Git.Revision,
							Found:             true,
						},
					},
				},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
				assert.Equal(t, *cmd.Git.URL, updated.Spec.ForProvider.Git.URL)
			},
			checkSecret: func(t *testing.T, cmd applicationCmd, authSecret *corev1.Secret) {
				assert.Equal(t, "fakekey", string(authSecret.Data[util.PrivateKeySecretKey]))
				assert.Equal(t, authSecret.Annotations[util.ManagedByAnnotation], util.NctlName)
			},
		},
		"disable deploy job": {
			orig: func() *apps.Application {
				a := existingApp.DeepCopy()
				a.Spec.ForProvider.Config.DeployJob = &apps.DeployJob{
					Job: apps.Job{
						Command: "date",
						Name:    "print-date",
					},
					FiniteJob: apps.FiniteJob{
						Retries: ptr.To(int32(2)),
						Timeout: &metav1.Duration{Duration: time.Minute},
					},
				}
				return a
			}(),
			gitAuth: &util.GitAuth{
				SSHPrivateKey: ptr.To("fakekey"),
			},
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				Git: &gitConfig{
					URL: ptr.To("https://newgit.example.org"),
				},
				baseConfig: baseConfig{
					DeployJob: &deployJob{Enabled: ptr.To(false)},
				},
			},
			gitInformationServiceResponse: test.GitInformationServiceResponse{
				Code: 200,
				Content: apps.GitExploreResponse{
					RepositoryInfo: &apps.RepositoryInfo{
						URL:      "https://newgit.example.org",
						Branches: []string{existingApp.Spec.ForProvider.Git.Revision},
						RevisionResponse: &apps.RevisionResponse{
							RevisionRequested: existingApp.Spec.ForProvider.Git.Revision,
							Found:             true,
						},
					},
				},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
			},
		},
		"remove worker job": {
			orig: func() *apps.Application {
				a := existingApp.DeepCopy()
				a.Spec.ForProvider.Config.WorkerJobs = []apps.WorkerJob{
					{
						Job:  apps.Job{Name: "do-0", Command: "echo 0"},
						Size: ptr.To(test.AppStandard1),
					},
				}
				return a
			}(),
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				baseConfig: baseConfig{
					DeleteWorkerJob: ptr.To("do-0"),
				},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
			},
		},
		// "remvoe scheduled job": {},
		"retry release": {
			orig: existingApp,
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				RetryRelease: ptr.To(true),
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assert.NotNil(t, util.EnvVarByName(updated.Spec.ForProvider.Config.Env, ReleaseTrigger))

				// strip out the ReleaseTrigger entry before comparing everything else:
				normalizedUpdatedConfig := updated.Spec.ForProvider.Config.DeepCopy()
				normalizedUpdatedConfig.Env = removeEnvVar(normalizedUpdatedConfig.Env, ReleaseTrigger)
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, *normalizedUpdatedConfig)
			},
		},
		"do not retry release": {
			orig: existingApp,
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				RetryRelease: ptr.To(false),
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assert.Nil(t, util.EnvVarByName(updated.Spec.ForProvider.Config.Env, ReleaseTrigger))
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
			},
		},
		"retry build": {
			orig: existingApp,
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				RetryBuild: ptr.To(true),
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assert.NotNil(t, util.EnvVarByName(updated.Spec.ForProvider.BuildEnv, BuildTrigger))
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
			},
		},
		"do not retry build": {
			orig: existingApp,
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				RetryBuild: ptr.To(false),
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assert.Nil(t, util.EnvVarByName(updated.Spec.ForProvider.BuildEnv, BuildTrigger))
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
			},
		},
		"disabling the git repo check works": {
			orig: existingApp,
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				Git: &gitConfig{
					URL: ptr.To("https://newgit.example.org"),
				},
				SkipRepoAccessCheck: true,
			},
			gitInformationServiceResponse: test.GitInformationServiceResponse{
				Code: 200,
				Content: apps.GitExploreResponse{
					Error: "repository can not be accessed",
				},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
				assert.Equal(t, *cmd.Git.URL, updated.Spec.ForProvider.Git.URL)
			},
		},
		"an error on the git repo check will lead to an error shown to the user": {
			orig: existingApp,
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				Git: &gitConfig{
					URL: ptr.To("https://newgit.example.org"),
				},
			},
			gitInformationServiceResponse: test.GitInformationServiceResponse{
				Code: 200,
				Content: apps.GitExploreResponse{
					Error: "repository can not be accessed",
				},
			},
			errorExpected: true,
		},
		"specifying a non existing branch/tag will be detected": {
			orig: existingApp,
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				Git: &gitConfig{
					URL:      ptr.To("https://newgit.example.org"),
					Revision: ptr.To("not-existent"),
				},
			},
			gitInformationServiceResponse: test.GitInformationServiceResponse{
				Code: 200,
				Content: apps.GitExploreResponse{
					RepositoryInfo: &apps.RepositoryInfo{
						URL:      "https://newgit.example.org",
						Branches: []string{"main"},
						RevisionResponse: &apps.RevisionResponse{
							RevisionRequested: "not-existent",
							Found:             false,
						},
					},
				},
			},
			errorExpected: true,
		},
		"defaulting to HTTPS when not specifying a scheme in a git URL works": {
			orig: existingApp,
			cmd: applicationCmd{
				resourceCmd: resourceCmd{
					Name: existingApp.Name,
				},
				Git: &gitConfig{
					URL:      ptr.To("github.com/ninech/new-repo"),
					Revision: ptr.To("main"),
				},
			},
			gitInformationServiceResponse: test.GitInformationServiceResponse{
				Code: 200,
				Content: apps.GitExploreResponse{
					RepositoryInfo: &apps.RepositoryInfo{
						URL:      "https://github.com/ninech/new-repo",
						Branches: []string{"main"},
						RevisionResponse: &apps.RevisionResponse{
							RevisionRequested: "main",
							Found:             true,
						},
					},
				},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
				assert.Equal(t, "https://github.com/ninech/new-repo", updated.Spec.ForProvider.Git.URL)
				assert.Equal(t, "main", updated.Spec.ForProvider.Git.Revision)
			},
		},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			if tc.cmd.GitInformationServiceURL == "" {
				tc.cmd.GitInformationServiceURL = gitInfoService.URL()
			}
			gitInfoService.SetResponse(tc.gitInformationServiceResponse)

			objects := []client.Object{tc.orig}
			if tc.gitAuth != nil {
				objects = append(objects, tc.gitAuth.Secret(tc.orig))
			}
			apiClient, err := test.SetupClient(
				test.WithObjects(objects...),
			)
			require.NoError(t, err)

			if err := tc.cmd.Run(ctx, apiClient); err != nil {
				if tc.errorExpected {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
			}

			updated := &apps.Application{}
			if err := apiClient.Get(ctx, apiClient.Name(tc.orig.Name), updated); err != nil {
				t.Fatal(err)
			}

			if tc.checkApp != nil {
				tc.checkApp(t, tc.cmd, tc.orig, updated)
			}

			if tc.checkSecret != nil {
				updatedSecret := &corev1.Secret{}
				if err := apiClient.Get(ctx, apiClient.Name(tc.orig.Name), updatedSecret); err != nil {
					t.Fatal(err)
				}

				tc.checkSecret(t, tc.cmd, updatedSecret)
			}
		})
	}
}

func newBaseConfigCmdAllFields() baseConfig {
	return baseConfig{
		Size:      ptr.To("newsize"),
		Port:      ptr.To(int32(1000)),
		Replicas:  ptr.To(int32(2)),
		Env:       map[string]string{"zoo": "bar"},
		BasicAuth: ptr.To(true),
		DeployJob: &deployJob{
			Command: ptr.To("exit 0"), Name: ptr.To("exit"),
			Retries: ptr.To(int32(1)), Timeout: ptr.To(time.Minute * 5),
		},
		WorkerJob: &workerJob{
			Name:    ptr.To("do-stuff-1"),
			Command: ptr.To("echo stuff1"),
			Size:    ptr.To(string(test.AppStandard1)),
		},
		ScheduledJob: &scheduledJob{
			Command:  ptr.To("sleep 5; date"),
			Name:     ptr.To("scheduled-1"),
			Size:     ptr.To(string(test.AppStandard1)),
			Schedule: ptr.To("* * * * *"),
			Retries:  ptr.To(int32(2)), Timeout: ptr.To(time.Minute * 5),
		},
	}
}

// TestApplicationFlags tests the behavior of kong's flag parser when using
// pointers. As we rely on pointers to check if a user supplied a flag we also
// want to test it in case this ever changes in future kong versions.
func TestApplicationFlags(t *testing.T) {
	nilFlags := &applicationCmd{}
	vars, err := create.ApplicationKongVars()
	require.NoError(t, err)
	_, err = kong.Must(nilFlags, vars).Parse([]string{`testname`})
	require.NoError(t, err)

	assert.Nil(t, nilFlags.Hosts)
	assert.Nil(t, nilFlags.Env)
	assert.Nil(t, nilFlags.BuildEnv)

	emptyFlags := &applicationCmd{}
	_, err = kong.Must(emptyFlags, vars).Parse([]string{`testname`, `--hosts=""`, `--env=`, `--build-env=`})
	require.NoError(t, err)

	assert.NotNil(t, emptyFlags.Hosts)
	assert.NotNil(t, emptyFlags.Env)
	assert.NotNil(t, emptyFlags.BuildEnv)
}

// removeEnvVar drops any entry whose Name matches key,
// returning a new slice (nil if it ends up empty).
func removeEnvVar(env []apps.EnvVar, key string) []apps.EnvVar {
	var out []apps.EnvVar
	for _, e := range env {
		if e.Name == key {
			continue
		}
		out = append(out, e)
	}
	return out
}
