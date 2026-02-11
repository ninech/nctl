package create

import (
	"testing"
	"time"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func TestProjectConfig(t *testing.T) {
	t.Parallel()

	apiClient := test.SetupClient(t)

	cases := map[string]struct {
		cmd         configCmd
		project     string
		checkConfig func(t *testing.T, cmd configCmd, cfg *apps.ProjectConfig)
	}{
		"all fields set": {
			cmd: configCmd{
				Size:      string(test.AppMini),
				Port:      ptr.To(int32(1337)),
				Replicas:  ptr.To(int32(42)),
				Env:       &map[string]string{"key1": "val1"},
				BasicAuth: ptr.To(true),
				DeployJob: deployJob{
					Command: "exit 0", Name: "exit",
					Retries: 1, Timeout: time.Minute * 5,
				},
			},
			project: "namespace-1",
			checkConfig: func(t *testing.T, cmd configCmd, cfg *apps.ProjectConfig) {
				is := require.New(t)
				is.Equal(apiClient.Project, cfg.Name)
				is.Equal(apps.ApplicationSize(cmd.Size), cfg.Spec.ForProvider.Config.Size)
				is.Equal(*cmd.Port, *cfg.Spec.ForProvider.Config.Port)
				is.Equal(*cmd.Replicas, *cfg.Spec.ForProvider.Config.Replicas)
				is.Equal(*cmd.BasicAuth, *cfg.Spec.ForProvider.Config.EnableBasicAuth)
				is.Equal(util.EnvVarsFromMap(*cmd.Env), cfg.Spec.ForProvider.Config.Env)
				is.Equal(cmd.DeployJob.Command, cfg.Spec.ForProvider.Config.DeployJob.Command)
				is.Equal(cmd.DeployJob.Name, cfg.Spec.ForProvider.Config.DeployJob.Name)
				is.Equal(cmd.DeployJob.Timeout, cfg.Spec.ForProvider.Config.DeployJob.Timeout.Duration)
				is.Equal(cmd.DeployJob.Retries, *cfg.Spec.ForProvider.Config.DeployJob.Retries)
			},
		},
		"some fields not set": {
			cmd: configCmd{
				Size:     string(test.AppMicro),
				Replicas: ptr.To(int32(1)),
			},
			project: "namespace-2",
			checkConfig: func(t *testing.T, cmd configCmd, cfg *apps.ProjectConfig) {
				is := require.New(t)
				is.Equal(apiClient.Project, cfg.Name)
				is.Equal(apps.ApplicationSize(cmd.Size), cfg.Spec.ForProvider.Config.Size)
				is.Nil(cfg.Spec.ForProvider.Config.Port)
				is.Nil(cfg.Spec.ForProvider.Config.EnableBasicAuth)
				is.Equal(*cmd.Replicas, *cfg.Spec.ForProvider.Config.Replicas)
				is.Empty(cfg.Spec.ForProvider.Config.Env)
				is.Nil(cfg.Spec.ForProvider.Config.DeployJob)
			},
		},
		"all fields not set": {
			cmd:     configCmd{},
			project: "namespace-3",
			checkConfig: func(t *testing.T, cmd configCmd, cfg *apps.ProjectConfig) {
				is := require.New(t)
				is.Equal(apiClient.Project, cfg.Name)
				is.Equal(test.AppSizeNotSet, cfg.Spec.ForProvider.Config.Size)
				is.Nil(cfg.Spec.ForProvider.Config.Port)
				is.Nil(cfg.Spec.ForProvider.Config.Replicas)
				is.Empty(cfg.Spec.ForProvider.Config.Env)
				is.Nil(cfg.Spec.ForProvider.Config.EnableBasicAuth)
				is.Nil(cfg.Spec.ForProvider.Config.DeployJob)
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			apiClient.Project = tc.project
			cfg := tc.cmd.newProjectConfig(tc.project)

			if err := tc.cmd.Run(t.Context(), apiClient); err != nil {
				t.Fatal(err)
			}

			if err := apiClient.Get(t.Context(), api.ObjectName(cfg), cfg); err != nil {
				t.Fatal(err)
			}

			tc.checkConfig(t, tc.cmd, cfg)
		})
	}
}
