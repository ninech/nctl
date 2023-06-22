package update

import (
	"context"
	"testing"

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
					Size:     initialSize,
					Replicas: pointer.Int32(1),
					Port:     pointer.Int32(1337),
					Env:      util.EnvVarsFromMap(map[string]string{"foo": "bar"}),
				},
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
				Size:     pointer.String("newsize"),
				Port:     pointer.Int32(1234),
				Replicas: pointer.Int32(999),
				Hosts:    &[]string{"one.example.org", "two.example.org"},
				Env:      &map[string]string{"bar": "zoo"},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, orig, updated *apps.Application) {
				assert.Equal(t, *cmd.Git.URL, updated.Spec.ForProvider.Git.URL)
				assert.Equal(t, *cmd.Git.SubPath, updated.Spec.ForProvider.Git.SubPath)
				assert.Equal(t, *cmd.Git.Revision, updated.Spec.ForProvider.Git.Revision)
				assert.Equal(t, apps.ApplicationSize(*cmd.Size), updated.Spec.ForProvider.Config.Size)
				assert.Equal(t, *cmd.Port, *updated.Spec.ForProvider.Config.Port)
				assert.Equal(t, *cmd.Replicas, *updated.Spec.ForProvider.Config.Replicas)
				assert.Equal(t, *cmd.Hosts, updated.Spec.ForProvider.Hosts)
				assert.Equal(t, util.EnvVarsFromMap(*cmd.Env), updated.Spec.ForProvider.Config.Env)
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

	emptyFlags := &applicationCmd{}
	_, err = kong.Must(emptyFlags).Parse([]string{`testname`, `--hosts=""`, `--env=`})
	require.NoError(t, err)

	assert.NotNil(t, emptyFlags.Hosts)
	assert.NotNil(t, emptyFlags.Env)
}
