package create

import (
	"context"
	"testing"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
				baseConfig: newBaseConfigCmdAllFields(),
			},
			project: "namespace-1",
			checkConfig: func(t *testing.T, cmd configCmd, cfg *apps.ProjectConfig) {
				assertBaseConfig(t, cmd.baseConfig, cfg.Spec.ForProvider.Config)
				assert.Equal(t, apiClient.Project, cfg.Name)
			},
		},
		"some fields not set": {
			cmd: configCmd{
				baseConfig: baseConfig{
					Size:     ptr.To(string(test.AppMicro)),
					Replicas: ptr.To(int32(1)),
					WorkerJob: workerJob{
						Name:    "do-stuff-2",
						Command: "echo stuff2",
						Size:    ptr.To(string(test.AppSizeNotSet)),
					},
				},
			},
			project: "namespace-2",
			checkConfig: func(t *testing.T, cmd configCmd, cfg *apps.ProjectConfig) {
				assertBaseConfig(t, cmd.baseConfig, cfg.Spec.ForProvider.Config)
				assert.Equal(t, apiClient.Project, cfg.Name)
			},
		},
		"all fields not set": {
			cmd:     configCmd{},
			project: "namespace-3",
			checkConfig: func(t *testing.T, cmd configCmd, cfg *apps.ProjectConfig) {
				assertBaseConfig(t, cmd.baseConfig, cfg.Spec.ForProvider.Config)
				assert.Equal(t, apiClient.Project, cfg.Name)
			},
		},
	}

	for name, tc := range cases {
		tc := tc
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

// assertBaseConfig verifies that apps.Config fields were correctly updated
// based on the provided CLI config. It checks both modified fields and ensures
// that unspecified fields remain unchanged.
func assertBaseConfig(t *testing.T, cmd baseConfig, got apps.Config) {
	t.Helper()

	assertBaseConfigCore(t, cmd, got)
	assertBaseConfigJobs(t, cmd, got)
}

// assertBaseConfigCore asserts that the "core" flags (baseConfig) in CLI config
// ended up in the apps.Config, and that Env collection is updated correctly. It
// also ensures that unspecified fields remain default.
func assertBaseConfigCore(t *testing.T, cmd baseConfig, got apps.Config) {
	t.Helper()

	if cmd.Size != nil {
		assert.Equal(t, apps.ApplicationSize(*cmd.Size), got.Size, "size")
	} else {
		assert.Equal(t, test.AppSizeNotSet, got.Size, "size (default)")
	}
	if cmd.Port != nil {
		assert.Equal(t, *cmd.Port, *got.Port, "port")
	} else {
		assert.Nil(t, got.Port, "port (default)")
	}
	if cmd.Replicas != nil {
		assert.Equal(t, *cmd.Replicas, *got.Replicas, "replicas")
	} else {
		assert.Nil(t, got.Replicas, "replicas (default)")
	}
	if cmd.BasicAuth != nil {
		assert.Equal(t, *cmd.BasicAuth, *got.EnableBasicAuth, "basic auth")
	} else {
		assert.Nil(t, got.EnableBasicAuth, "basic auth (default)")
	}
	if cmd.Env != nil {
		expectedEnv := util.EnvVarsFromMap(cmd.Env)
		assert.Equal(t, expectedEnv, got.Env, "env vars")
	} else {
		assert.Empty(t, got.Env, "env vars (default)")
	}
}

// assertBaseConfigJobs provides a default logic for jobs assert.  It checks
// both modified fields and ensures that unspecified fields remain default.
func assertBaseConfigJobs(t *testing.T, cmd baseConfig, got apps.Config) {
	t.Helper()

	wantDeployJobs := deployJobsFromCmdNormalized(cmd.DeployJob)
	gotDeployJobs := test.PtrToSlice(got.DeployJob, func(d *apps.DeployJob) apps.DeployJob { return *d })
	test.AssertJobsEqual(t, wantDeployJobs, gotDeployJobs, func(d apps.DeployJob) test.DeployJobKey { return test.ToDeployJobKey(&d) })
	wantWorkerJobs := test.PtrToSlice(workerJobPtr(cmd.WorkerJob), workerJobFromCmd)
	gotWorkerJobs := test.NormalizeSlice(got.WorkerJobs)
	test.AssertJobsEqual(t, wantWorkerJobs, gotWorkerJobs, test.ToWorkerJobKey)
	wantScheduledJobs := test.PtrToSlice(scheduledJobPtr(cmd.ScheduledJob), scheduledJobFromCmd)
	gotScheduledJobs := test.NormalizeSlice(got.ScheduledJobs)
	test.AssertJobsEqual(t, wantScheduledJobs, gotScheduledJobs, test.ToScheduledJobKey)
}

// deployJobsFromCmdNormalized converts the CLI deployJob into the slice of CRD DeployJob.
func deployJobsFromCmdNormalized(j deployJob) []apps.DeployJob {
	if len(j.Command) == 0 || len(j.Name) == 0 {
		return nil
	}
	dj := apps.DeployJob{
		Job: apps.Job{
			Name:    j.Name,
			Command: j.Command,
		},
		FiniteJob: apps.FiniteJob{
			Retries: ptr.To(j.Retries),
			Timeout: &metav1.Duration{Duration: j.Timeout},
		},
	}
	return []apps.DeployJob{dj}
}

// workerJobFromCmd converts the CLI workerJob into the CRD WorkerJob.
func workerJobFromCmd(j *workerJob) apps.WorkerJob {
	if j == nil {
		return apps.WorkerJob{}
	}
	return apps.WorkerJob{
		Job: apps.Job{
			Name:    j.Name,
			Command: j.Command,
		},
		Size: ptr.To(apps.ApplicationSize(ptr.Deref(j.Size, ""))),
	}
}

// scheduledJobFromCmd turns the CLI representation into the form that finally
// ends up on the ProjectConfig after the mapping/conversion logic.
func scheduledJobFromCmd(j *scheduledJob) apps.ScheduledJob {
	if j == nil {
		return apps.ScheduledJob{}
	}
	return apps.ScheduledJob{
		Job: apps.Job{
			Name:    j.Name,
			Command: j.Command,
		},
		Size:     ptr.To(apps.ApplicationSize(ptr.Deref(j.Size, ""))),
		Schedule: j.Schedule,
		FiniteJob: apps.FiniteJob{
			Retries: ptr.To(j.Retries),
			Timeout: &metav1.Duration{Duration: j.Timeout},
		},
	}
}

// workerJobPtr returns nil if no or invalid --worker-job- flags were passed,
// or &j otherwise.
func workerJobPtr(j workerJob) *workerJob {
	if len(j.Command) == 0 || len(j.Name) == 0 {
		return nil
	}
	return &j
}

// scheduledJobPtr returns nil if no or invalid --scheduled-job- flags were passed,
// or &j otherwise.
func scheduledJobPtr(j scheduledJob) *scheduledJob {
	if len(j.Command) == 0 || len(j.Name) == 0 {
		return nil
	}
	return &j
}
