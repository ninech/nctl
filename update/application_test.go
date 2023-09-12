package update

import (
	"context"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestApplication(t *testing.T) {
	scheme, err := api.NewScheme()
	if err != nil {
		t.Fatal(err)
	}

	initialSize := apps.ApplicationSize("micro")
	existingApp := &apps.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-name",
			Namespace: "default",
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
					Replicas:        pointer.Int32(1),
					Port:            pointer.Int32(1337),
					Env:             util.EnvVarsFromMap(map[string]string{"foo": "bar"}),
					EnableBasicAuth: pointer.Bool(false),
					DeployJob: &apps.DeployJob{
						Job: apps.Job{
							Command: "date",
							Name:    "print-date",
						},
						FiniteJob: apps.FiniteJob{
							Retries: pointer.Int32(2),
							Timeout: &metav1.Duration{Duration: time.Minute},
						},
					},
				},
				BuildEnv: util.EnvVarsFromMap(map[string]string{"BP_ENVIRONMENT_VARIABLE": "some-value"}),
			},
		},
	}

	cases := map[string]struct {
		orig        *apps.Application
		gitAuth     util.GitAuth
		cmd         applicationCmd
		checkApp    func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application)
		checkSecret func(t *testing.T, cmd applicationCmd, authSecret *corev1.Secret)
	}{
		"change port": {
			orig: existingApp,
			cmd: applicationCmd{
				Name: pointer.String(existingApp.Name),
				Port: pointer.Int32(1234),
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assert.Equal(t, *cmd.Port, *updated.Spec.ForProvider.Config.Port)
			},
		},
		"port is unchanged when updating unrelated field": {
			orig: existingApp,
			cmd: applicationCmd{
				Name: pointer.String(existingApp.Name),
				Size: pointer.String("newsize"),
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assert.Equal(t, *orig.Spec.ForProvider.Config.Port, *updated.Spec.ForProvider.Config.Port)
				assert.NotEqual(t, orig.Spec.ForProvider.Config.Size, updated.Spec.ForProvider.Config.Size)
			},
		},
		"all field updates": {
			orig: existingApp,
			cmd: applicationCmd{
				Name: pointer.String(existingApp.Name),
				Git: &gitConfig{
					URL:      pointer.String("https://newgit.example.org"),
					SubPath:  pointer.String("new/path"),
					Revision: pointer.String("some-change"),
				},
				Size:      pointer.String("newsize"),
				Port:      pointer.Int32(1234),
				Replicas:  pointer.Int32(999),
				Hosts:     &[]string{"one.example.org", "two.example.org"},
				Env:       &map[string]string{"bar": "zoo"},
				BuildEnv:  &map[string]string{"BP_GO_TARGETS": "./cmd/web-server"},
				BasicAuth: pointer.Bool(true),
				DeployJob: &deployJob{
					Command: pointer.String("exit 0"), Name: pointer.String("exit"),
					Retries: pointer.Int32(1), Timeout: pointer.Duration(time.Minute * 5),
				},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assert.Equal(t, *cmd.Git.URL, updated.Spec.ForProvider.Git.URL)
				assert.Equal(t, *cmd.Git.SubPath, updated.Spec.ForProvider.Git.SubPath)
				assert.Equal(t, *cmd.Git.Revision, updated.Spec.ForProvider.Git.Revision)
				assert.Equal(t, apps.ApplicationSize(*cmd.Size), updated.Spec.ForProvider.Config.Size)
				assert.Equal(t, *cmd.Port, *updated.Spec.ForProvider.Config.Port)
				assert.Equal(t, *cmd.Replicas, *updated.Spec.ForProvider.Config.Replicas)
				assert.Equal(t, *cmd.BasicAuth, *updated.Spec.ForProvider.Config.EnableBasicAuth)
				assert.Equal(t, *cmd.Hosts, updated.Spec.ForProvider.Hosts)
				assert.Equal(t, util.UpdateEnvVars(existingApp.Spec.ForProvider.Config.Env, *cmd.Env, nil), updated.Spec.ForProvider.Config.Env)
				assert.Equal(t, util.UpdateEnvVars(existingApp.Spec.ForProvider.BuildEnv, *cmd.BuildEnv, nil), updated.Spec.ForProvider.BuildEnv)
				assert.Equal(t, *cmd.DeployJob.Command, updated.Spec.ForProvider.Config.DeployJob.Command)
				assert.Equal(t, *cmd.DeployJob.Name, updated.Spec.ForProvider.Config.DeployJob.Name)
				assert.Equal(t, *cmd.DeployJob.Timeout, updated.Spec.ForProvider.Config.DeployJob.Timeout.Duration)
				assert.Equal(t, *cmd.DeployJob.Retries, *updated.Spec.ForProvider.Config.DeployJob.Retries)
			},
		},
		"reset env variables": {
			orig: existingApp,
			cmd: applicationCmd{
				Name:           pointer.String(existingApp.Name),
				DeleteEnv:      &[]string{"foo"},
				DeleteBuildEnv: &[]string{"BP_ENVIRONMENT_VARIABLE"},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assert.Equal(t, apps.EnvVars{}, updated.Spec.ForProvider.Config.Env)
				assert.Equal(t, apps.EnvVars{}, updated.Spec.ForProvider.BuildEnv)
			},
		},
		"git auth update user/pass": {
			orig: existingApp,
			gitAuth: util.GitAuth{
				Username: pointer.String("some-user"),
				Password: pointer.String("some-password"),
			},
			cmd: applicationCmd{
				Name: pointer.String(existingApp.Name),
				Git: &gitConfig{
					Username: pointer.String("new-user"),
					Password: pointer.String("new-pass"),
				},
			},
			checkSecret: func(t *testing.T, cmd applicationCmd, authSecret *corev1.Secret) {
				assert.Equal(t, *cmd.Git.Username, string(authSecret.Data[util.UsernameSecretKey]))
				assert.Equal(t, *cmd.Git.Password, string(authSecret.Data[util.PasswordSecretKey]))
				assert.Equal(t, authSecret.Annotations[util.ManagedByAnnotation], util.NctlName)
			},
		},
		"git auth update ssh key": {
			orig: existingApp,
			gitAuth: util.GitAuth{
				SSHPrivateKey: pointer.String("fakekey"),
			},
			cmd: applicationCmd{
				Name: pointer.String(existingApp.Name),
				Git: &gitConfig{
					SSHPrivateKey: pointer.String("newfakekey"),
				},
			},
			checkSecret: func(t *testing.T, cmd applicationCmd, authSecret *corev1.Secret) {
				assert.Equal(t, *cmd.Git.SSHPrivateKey, string(authSecret.Data[util.PrivateKeySecretKey]))
				assert.Equal(t, authSecret.Annotations[util.ManagedByAnnotation], util.NctlName)
			},
		},
		"git auth is unchanged on normal field update": {
			orig: existingApp,
			gitAuth: util.GitAuth{
				SSHPrivateKey: pointer.String("fakekey"),
			},
			cmd: applicationCmd{
				Name: pointer.String(existingApp.Name),
				Git: &gitConfig{
					URL: pointer.String("https://newgit.example.org"),
				},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assert.Equal(t, *cmd.Git.URL, updated.Spec.ForProvider.Git.URL)
			},
			checkSecret: func(t *testing.T, cmd applicationCmd, authSecret *corev1.Secret) {
				assert.Equal(t, "fakekey", string(authSecret.Data[util.PrivateKeySecretKey]))
				assert.Equal(t, authSecret.Annotations[util.ManagedByAnnotation], util.NctlName)
			},
		},
		"disable deploy job": {
			orig: existingApp,
			gitAuth: util.GitAuth{
				SSHPrivateKey: pointer.String("fakekey"),
			},
			cmd: applicationCmd{
				Name: pointer.String(existingApp.Name),
				Git: &gitConfig{
					URL: pointer.String("https://newgit.example.org"),
				},
				DeployJob: &deployJob{Enabled: pointer.Bool(false)},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assert.Nil(t, updated.Spec.ForProvider.Config.DeployJob)
			},
		},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
				tc.orig, tc.gitAuth.Secret(tc.orig),
			).Build()
			apiClient := &api.Client{WithWatch: client, Project: "default"}
			ctx := context.Background()

			if err := tc.cmd.Run(ctx, apiClient); err != nil {
				t.Fatal(err)
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

// TestApplicationFlags tests the behavior of kong's flag parser when using
// pointers. As we rely on pointers to check if a user supplied a flag we also
// want to test it in case this ever changes in future kong versions.
func TestApplicationFlags(t *testing.T) {
	nilFlags := &applicationCmd{}
	_, err := kong.Must(nilFlags).Parse([]string{`testname`})
	require.NoError(t, err)

	assert.Nil(t, nilFlags.Hosts)
	assert.Nil(t, nilFlags.Env)
	assert.Nil(t, nilFlags.BuildEnv)

	emptyFlags := &applicationCmd{}
	_, err = kong.Must(emptyFlags).Parse([]string{`testname`, `--hosts=""`, `--env=`, `--build-env=`})
	require.NoError(t, err)

	assert.NotNil(t, emptyFlags.Hosts)
	assert.NotNil(t, emptyFlags.Env)
	assert.NotNil(t, emptyFlags.BuildEnv)
}
