package update

import (
	"context"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestConfig(t *testing.T) {
	const project = "some-project"

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
					Replicas:        ptr.To(int32(1)),
					Port:            ptr.To(int32(1337)),
					Env:             util.EnvVarsFromMap(map[string]string{"foo": "bar"}),
					EnableBasicAuth: ptr.To(false),
				},
			},
		},
	}

	cases := map[string]struct {
		orig        *apps.ProjectConfig
		project     string
		cmd         configCmd
		checkConfig func(t *testing.T, cmd configCmd, orig, updated *apps.ProjectConfig)
	}{
		"change port": {
			orig:    existingConfig,
			project: project,
			cmd: configCmd{
				Port: ptr.To(int32(1234)),
			},
			checkConfig: func(t *testing.T, cmd configCmd, orig, updated *apps.ProjectConfig) {
				assert.Equal(t, *cmd.Port, *updated.Spec.ForProvider.Config.Port)
			},
		},
		"port is unchanged when updating unrelated field": {
			orig:    existingConfig,
			project: project,
			cmd: configCmd{
				Size: ptr.To("newsize"),
			},
			checkConfig: func(t *testing.T, cmd configCmd, orig, updated *apps.ProjectConfig) {
				assert.Equal(t, *orig.Spec.ForProvider.Config.Port, *updated.Spec.ForProvider.Config.Port)
				assert.NotEqual(t, orig.Spec.ForProvider.Config.Size, updated.Spec.ForProvider.Config.Size)
			},
		},
		"update basic auth": {
			orig:    existingConfig,
			project: project,
			cmd: configCmd{
				BasicAuth: ptr.To(true),
			},
			checkConfig: func(t *testing.T, cmd configCmd, orig, updated *apps.ProjectConfig) {
				assert.True(t, *updated.Spec.ForProvider.Config.EnableBasicAuth)
			},
		},
		"all fields update": {
			orig:    existingConfig,
			project: project,
			cmd: configCmd{
				Size:      ptr.To("newsize"),
				Port:      ptr.To(int32(1000)),
				Replicas:  ptr.To(int32(2)),
				Env:       map[string]string{"zoo": "bar"},
				BasicAuth: ptr.To(true),
				DeployJob: &deployJob{
					Command: ptr.To("exit 0"), Name: ptr.To("exit"),
					Retries: ptr.To(int32(1)), Timeout: ptr.To(time.Minute * 5),
				},
			},
			checkConfig: func(t *testing.T, cmd configCmd, orig, updated *apps.ProjectConfig) {
				assert.Equal(t, apps.ApplicationSize(*cmd.Size), updated.Spec.ForProvider.Config.Size)
				assert.Equal(t, *cmd.Port, *updated.Spec.ForProvider.Config.Port)
				assert.Equal(t, *cmd.Replicas, *updated.Spec.ForProvider.Config.Replicas)
				assert.Equal(t, *cmd.BasicAuth, *updated.Spec.ForProvider.Config.EnableBasicAuth)
				assert.Equal(t, util.EnvVarsFromMap(cmd.Env), updated.Spec.ForProvider.Config.Env)
				assert.Equal(t, *cmd.DeployJob.Command, updated.Spec.ForProvider.Config.DeployJob.Command)
				assert.Equal(t, *cmd.DeployJob.Name, updated.Spec.ForProvider.Config.DeployJob.Name)
				assert.Equal(t, *cmd.DeployJob.Timeout, updated.Spec.ForProvider.Config.DeployJob.Timeout.Duration)
				assert.Equal(t, *cmd.DeployJob.Retries, *updated.Spec.ForProvider.Config.DeployJob.Retries)
			},
		},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			apiClient, err := test.SetupClient(tc.orig)
			if err != nil {
				t.Fatal(err)
			}
			apiClient.Project = tc.project

			ctx := context.Background()

			if err := tc.cmd.Run(ctx, apiClient); err != nil {
				t.Fatal(err)
			}

			updated := &apps.ProjectConfig{}
			if err := apiClient.Get(ctx, apiClient.Name(tc.orig.Name), updated); err != nil {
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
	nilFlags := &configCmd{}
	_, err := kong.Must(nilFlags).Parse([]string{})
	require.NoError(t, err)

	assert.Nil(t, nilFlags.Env)

	emptyFlags := &configCmd{}
	_, err = kong.Must(emptyFlags).Parse([]string{`--env=`})
	require.NoError(t, err)

	assert.NotNil(t, emptyFlags.Env)
}
