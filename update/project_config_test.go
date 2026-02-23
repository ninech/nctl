package update

import (
	"testing"
	"time"

	"github.com/alecthomas/kong"
	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/internal/application"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestConfig(t *testing.T) {
	t.Parallel()

	const project = test.DefaultProject

	initialSize := test.AppMicro

	existingConfig := &apps.ProjectConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      project,
			Namespace: project,
		},
		Spec: apps.ProjectConfigSpec{
			ForProvider: apps.ProjectConfigParameters{
				Config: apps.Config{
					Size:            initialSize,
					Replicas:        new(int32(1)),
					Port:            new(int32(1337)),
					Env:             application.EnvVarsFromMap(map[string]string{"foo": "bar"}),
					EnableBasicAuth: new(false),
				},
			},
		},
	}

	cases := map[string]struct {
		orig        *apps.ProjectConfig
		cmd         configCmd
		checkConfig func(t *testing.T, cmd configCmd, orig, updated *apps.ProjectConfig)
	}{
		"change port": {
			orig: existingConfig,
			cmd: configCmd{
				Port: new(int32(1234)),
			},
			checkConfig: func(t *testing.T, cmd configCmd, orig, updated *apps.ProjectConfig) {
				is := require.New(t)
				is.Equal(*cmd.Port, *updated.Spec.ForProvider.Config.Port)
			},
		},
		"port is unchanged when updating unrelated field": {
			orig: existingConfig,
			cmd: configCmd{
				Size: new("newsize"),
			},
			checkConfig: func(t *testing.T, cmd configCmd, orig, updated *apps.ProjectConfig) {
				is := require.New(t)
				is.Equal(*orig.Spec.ForProvider.Config.Port, *updated.Spec.ForProvider.Config.Port)
				is.NotEqual(orig.Spec.ForProvider.Config.Size, updated.Spec.ForProvider.Config.Size)
			},
		},
		"update basic auth": {
			orig: existingConfig,
			cmd: configCmd{
				BasicAuth: new(true),
			},
			checkConfig: func(t *testing.T, cmd configCmd, orig, updated *apps.ProjectConfig) {
				is := require.New(t)
				is.True(*updated.Spec.ForProvider.Config.EnableBasicAuth)
			},
		},
		"all fields update": {
			orig: existingConfig,
			cmd: configCmd{
				Size:      new("newsize"),
				Port:      new(int32(1000)),
				Replicas:  new(int32(2)),
				Env:       map[string]string{"zoo": "bar"},
				BasicAuth: new(true),
				DeployJob: &deployJob{
					Command: new("exit 0"), Name: new("exit"),
					Retries: new(int32(1)), Timeout: ptr.To(time.Minute * 5),
				},
			},
			checkConfig: func(t *testing.T, cmd configCmd, orig, updated *apps.ProjectConfig) {
				is := require.New(t)
				is.Equal(apps.ApplicationSize(*cmd.Size), updated.Spec.ForProvider.Config.Size)
				is.Equal(*cmd.Port, *updated.Spec.ForProvider.Config.Port)
				is.Equal(*cmd.Replicas, *updated.Spec.ForProvider.Config.Replicas)
				is.Equal(*cmd.BasicAuth, *updated.Spec.ForProvider.Config.EnableBasicAuth)
				is.Equal(application.EnvVarsFromMap(cmd.Env), updated.Spec.ForProvider.Config.Env)
				is.Equal(*cmd.DeployJob.Command, updated.Spec.ForProvider.Config.DeployJob.Command)
				is.Equal(*cmd.DeployJob.Name, updated.Spec.ForProvider.Config.DeployJob.Name)
				is.Equal(*cmd.DeployJob.Timeout, updated.Spec.ForProvider.Config.DeployJob.Timeout.Duration)
				is.Equal(*cmd.DeployJob.Retries, *updated.Spec.ForProvider.Config.DeployJob.Retries)
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			apiClient := test.SetupClient(t,
				test.WithObjects(tc.orig),
			)

			if err := tc.cmd.Run(t.Context(), apiClient); err != nil {
				t.Fatal(err)
			}

			updated := &apps.ProjectConfig{}
			if err := apiClient.Get(t.Context(), apiClient.Name(tc.orig.Name), updated); err != nil {
				t.Fatal(err)
			}

			if tc.checkConfig != nil {
				tc.checkConfig(t, tc.cmd, tc.orig, updated)
			}
		})
	}
}

// TestProjectConfigFlags tests the behavior of kong's flag parser when using
// pointers. As we rely on pointers to check if a user supplied a flag we also
// want to test it in case this ever changes in future kong versions.
func TestProjectConfigFlags(t *testing.T) {
	t.Parallel()

	is := require.New(t)

	type cmd struct {
		Env map[string]string `help:"Environment variables which are passed to the app at runtime."`
	}
	nilFlags := &cmd{}
	_, err := kong.Must(nilFlags).Parse([]string{})
	is.NoError(err)

	is.Nil(nilFlags.Env)

	emptyFlags := &cmd{}
	_, err = kong.Must(emptyFlags).Parse([]string{`--env=`})
	is.NoError(err)

	is.NotNil(emptyFlags.Env)
}
