package create

import (
	"context"
	"testing"
	"time"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/ptr"
)

func TestProjectConfig(t *testing.T) {
	apiClient, err := test.SetupClient()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

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
				assert.Equal(t, apiClient.Project, cfg.Name)
				assert.Equal(t, apps.ApplicationSize(cmd.Size), cfg.Spec.ForProvider.Config.Size)
				assert.Equal(t, *cmd.Port, *cfg.Spec.ForProvider.Config.Port)
				assert.Equal(t, *cmd.Replicas, *cfg.Spec.ForProvider.Config.Replicas)
				assert.Equal(t, *cmd.BasicAuth, *cfg.Spec.ForProvider.Config.EnableBasicAuth)
				assert.Equal(t, util.EnvVarsFromMap(*cmd.Env), cfg.Spec.ForProvider.Config.Env)
				assert.Equal(t, cmd.DeployJob.Command, cfg.Spec.ForProvider.Config.DeployJob.Command)
				assert.Equal(t, cmd.DeployJob.Name, cfg.Spec.ForProvider.Config.DeployJob.Name)
				assert.Equal(t, cmd.DeployJob.Timeout, cfg.Spec.ForProvider.Config.DeployJob.Timeout.Duration)
				assert.Equal(t, cmd.DeployJob.Retries, *cfg.Spec.ForProvider.Config.DeployJob.Retries)
			},
		},
		"some fields not set": {
			cmd: configCmd{
				Size:     string(test.AppMicro),
				Replicas: ptr.To(int32(1)),
			},
			project: "namespace-2",
			checkConfig: func(t *testing.T, cmd configCmd, cfg *apps.ProjectConfig) {
				assert.Equal(t, apiClient.Project, cfg.Name)
				assert.Equal(t, apps.ApplicationSize(cmd.Size), cfg.Spec.ForProvider.Config.Size)
				assert.Nil(t, cfg.Spec.ForProvider.Config.Port)
				assert.Nil(t, cfg.Spec.ForProvider.Config.EnableBasicAuth)
				assert.Equal(t, *cmd.Replicas, *cfg.Spec.ForProvider.Config.Replicas)
				assert.Empty(t, cfg.Spec.ForProvider.Config.Env)
				assert.Nil(t, cfg.Spec.ForProvider.Config.DeployJob)
			},
		},
		"all fields not set": {
			cmd:     configCmd{},
			project: "namespace-3",
			checkConfig: func(t *testing.T, cmd configCmd, cfg *apps.ProjectConfig) {
				assert.Equal(t, apiClient.Project, cfg.Name)
				assert.Equal(t, test.AppSizeNotSet, cfg.Spec.ForProvider.Config.Size)
				assert.Nil(t, cfg.Spec.ForProvider.Config.Port)
				assert.Nil(t, cfg.Spec.ForProvider.Config.Replicas)
				assert.Empty(t, cfg.Spec.ForProvider.Config.Env)
				assert.Nil(t, cfg.Spec.ForProvider.Config.EnableBasicAuth)
				assert.Nil(t, cfg.Spec.ForProvider.Config.DeployJob)
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			apiClient.Project = tc.project
			cfg := tc.cmd.newProjectConfig(tc.project)

			if err := tc.cmd.Run(ctx, apiClient); err != nil {
				t.Fatal(err)
			}

			if err := apiClient.Get(ctx, api.ObjectName(cfg), cfg); err != nil {
				t.Fatal(err)
			}

			tc.checkConfig(t, tc.cmd, cfg)
		})
	}
}
