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
		cmd         configCmd
		checkConfig func(t *testing.T, cmd configCmd, orig, updated *apps.ProjectConfig)
	}{
		"change port": {
			orig: existingConfig,
			cmd: configCmd{
				baseConfig: baseConfig{
					Port: ptr.To(int32(1234)),
				},
			},
			checkConfig: func(t *testing.T, cmd configCmd, orig, updated *apps.ProjectConfig) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
			},
		},
		"port is unchanged when updating unrelated field": {
			orig: existingConfig,
			cmd: configCmd{
				baseConfig: baseConfig{
					Size: ptr.To("newsize"),
				},
			},
			checkConfig: func(t *testing.T, cmd configCmd, orig, updated *apps.ProjectConfig) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
			},
		},
		"update basic auth": {
			orig: existingConfig,
			cmd: configCmd{
				baseConfig: baseConfig{
					BasicAuth: ptr.To(true),
				},
			},
			checkConfig: func(t *testing.T, cmd configCmd, orig, updated *apps.ProjectConfig) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
			},
		},
		"all fields update": {
			orig: existingConfig,
			cmd:  configCmd{newBaseConfigCmdAllFields()},
			checkConfig: func(t *testing.T, cmd configCmd, orig, updated *apps.ProjectConfig) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
			},
		},
		"reset env variable": {
			orig: existingConfig,
			cmd: configCmd{
				baseConfig: baseConfig{
					DeleteEnv: &[]string{"foo"},
				},
			},
			checkConfig: func(t *testing.T, cmd configCmd, orig, updated *apps.ProjectConfig) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
			},
		},

		"jobs union adds distinct element": {
			orig: func() *apps.ProjectConfig {
				c := existingConfig.DeepCopy()
				c.Spec.ForProvider.Config.WorkerJobs = []apps.WorkerJob{
					{
						Job: apps.Job{
							Name:    "do-stuff-0",
							Command: "echo stuff0",
						},
						Size: ptr.To(test.AppStandard1),
					},
				}
				c.Spec.ForProvider.Config.ScheduledJobs = []apps.ScheduledJob{
					{
						Job: apps.Job{
							Command: "sleep 1; date",
							Name:    "scheduled-0a",
						},
						Size:     ptr.To(test.AppMini),
						Schedule: "* * * * *",
						FiniteJob: apps.FiniteJob{
							Retries: ptr.To(int32(2)),
							Timeout: &metav1.Duration{Duration: time.Minute * 5},
						},
					},
					{
						Job: apps.Job{
							Command: "sleep 2; date",
							Name:    "scheduled-0b",
						},
					},
				}
				return c
			}(),
			cmd: configCmd{
				baseConfig: baseConfig{
					Size:      ptr.To("newsize"),
					Port:      ptr.To(int32(1000)),
					Replicas:  ptr.To(int32(2)),
					Env:       map[string]string{"zoo": "bar"},
					BasicAuth: ptr.To(true),
					WorkerJob: &workerJob{
						Name:    ptr.To("do-stuff-1"),
						Command: ptr.To("echo stuff1"),
						Size:    ptr.To(string(test.AppStandard1)),
					},
					ScheduledJob: &scheduledJob{
						Command:  ptr.To("sleep 11; date"),
						Name:     ptr.To("scheduled-1"),
						Size:     ptr.To(string(test.AppStandard1)),
						Schedule: ptr.To("1 * * * *"),
						Retries:  ptr.To(int32(2)), Timeout: ptr.To(time.Minute * 5),
					},
				},
			},
			checkConfig: func(t *testing.T, cmd configCmd, orig, updated *apps.ProjectConfig) {
				assertBaseConfig(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)
			},
		},
		"jobs union modifies existing element": {
			orig: func() *apps.ProjectConfig {
				c := existingConfig.DeepCopy()
				c.Spec.ForProvider.Config.DeployJob = &apps.DeployJob{
					Job: apps.Job{Name: "init-0", Command: "sleep 1"},
					FiniteJob: apps.FiniteJob{
						Retries: ptr.To(int32(1)),
						Timeout: &metav1.Duration{Duration: time.Second * 10},
					},
				}
				c.Spec.ForProvider.Config.WorkerJobs = []apps.WorkerJob{
					{
						Job:  apps.Job{Name: "work-0", Command: "sleep 2"},
						Size: ptr.To(test.AppStandard1),
					},
				}
				c.Spec.ForProvider.Config.ScheduledJobs = []apps.ScheduledJob{
					{
						Job:      apps.Job{Name: "sch-0", Command: "sleep 3"},
						Size:     ptr.To(test.AppMini),
						Schedule: "* * * * *",
						FiniteJob: apps.FiniteJob{
							Retries: ptr.To(int32(2)),
							Timeout: &metav1.Duration{Duration: time.Minute * 5},
						},
					},
				}
				return c
			}(),
			cmd: configCmd{
				baseConfig: baseConfig{
					// note: we modify existing jobs here:
					DeployJob: &deployJob{
						Name: ptr.To("init-0"), Command: ptr.To("echo 1"),
						Timeout: ptr.To(time.Second * 3),
					},
					WorkerJob: &workerJob{
						Name: ptr.To("work-0"),
						Size: ptr.To(string(test.AppMicro)),
					},
					ScheduledJob: &scheduledJob{
						Name:     ptr.To("sch-0"),
						Command:  ptr.To("sleep 99"),
						Schedule: ptr.To("3 * * * *"),
					},
				},
			},
			checkConfig: func(t *testing.T, cmd configCmd, orig, updated *apps.ProjectConfig) {
				// the basic configuration should not change, only the jobs are modified here:
				assertBaseConfigCore(t, orig.Spec.ForProvider.Config, cmd.baseConfig, updated.Spec.ForProvider.Config)

				d := orig.Spec.ForProvider.Config.DeployJob.DeepCopy()
				d.Command = "echo 1"
				d.Timeout = &metav1.Duration{Duration: time.Second * 3}
				wantDeployJobs := []apps.DeployJob{*d}
				gotDeployJobs := test.PtrToSlice(updated.Spec.ForProvider.Config.DeployJob, func(d *apps.DeployJob) apps.DeployJob { return *d })
				test.AssertJobsEqual(t, wantDeployJobs, gotDeployJobs, func(d apps.DeployJob) test.DeployJobKey { return test.ToDeployJobKey(&d) })

				w := orig.Spec.ForProvider.Config.WorkerJobs[0]
				w.Size = ptr.To(test.AppMicro)
				wantWorkerJobs := []apps.WorkerJob{w}
				gotWorkerJobs := test.NormalizeSlice(updated.Spec.ForProvider.Config.WorkerJobs)
				test.AssertJobsEqual(t, wantWorkerJobs, gotWorkerJobs, test.ToWorkerJobKey)

				j := orig.Spec.ForProvider.Config.ScheduledJobs[0]
				j.Command = "sleep 99"
				j.Schedule = "3 * * * *"
				wantScheduledJobs := []apps.ScheduledJob{j}
				gotScheduledJobs := test.NormalizeSlice(updated.Spec.ForProvider.Config.ScheduledJobs)
				test.AssertJobsEqual(t, wantScheduledJobs, gotScheduledJobs, test.ToScheduledJobKey)
			},
		},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			apiClient, err := test.SetupClient(
				test.WithObjects(tc.orig),
			)
			if err != nil {
				t.Fatal(err)
			}

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

// assertBaseConfig verifies that apps.Config fields were correctly updated
// based on the provided CLI config. It checks both modified fields and ensures
// that unspecified fields remain unchanged.
func assertBaseConfig(t *testing.T, orig apps.Config, cmd baseConfig, got apps.Config) {
	t.Helper()

	assertBaseConfigCore(t, orig, cmd, got)
	assertBaseConfigJobs(t, orig, cmd, got)
}

// assertBaseConfigCore asserts that the "core" flags (baseConfig) in CLI config
// ended up in the updated config, and that Env collection is updated correctly.
// It also ensures that unspecified fields remain unchanged.
func assertBaseConfigCore(t *testing.T, orig apps.Config, cmd baseConfig, got apps.Config) {
	t.Helper()

	if cmd.Size != nil {
		assert.Equal(t, apps.ApplicationSize(*cmd.Size), got.Size, "size")
	} else {
		assert.Equal(t, orig.Size, got.Size, "size (unchanged)")
	}
	if cmd.Port != nil {
		assert.Equal(t, *cmd.Port, *got.Port, "port")
	} else {
		assert.Equal(t, *orig.Port, *got.Port, "port (unchanged)")
	}
	if cmd.Replicas != nil {
		assert.Equal(t, *cmd.Replicas, *got.Replicas, "replicas")
	} else {
		assert.Equal(t, *orig.Replicas, *got.Replicas, "replicas (unchanged)")
	}
	if cmd.BasicAuth != nil {
		assert.Equal(t, *cmd.BasicAuth, *got.EnableBasicAuth, "basic auth")
	} else {
		assert.Equal(t, *orig.EnableBasicAuth, *got.EnableBasicAuth, "basic auth (unchanged)")
	}
	if cmd.Env != nil || cmd.DeleteEnv != nil {
		runtimeEnv := map[string]string{}
		if cmd.Env != nil {
			runtimeEnv = cmd.Env
		}
		var deleteEnv []string
		if cmd.DeleteEnv != nil {
			deleteEnv = *cmd.DeleteEnv
		}
		expectedEnv := util.UpdateEnvVars(orig.Env, runtimeEnv, deleteEnv)
		assert.Equal(t, expectedEnv, got.Env, "env vars (added and/or deleted)")
	} else {
		assert.Equal(t,
			orig.Env, got.Env, "env vars (unchanged)")
	}
}

// assertBaseConfigJobs provides a default logic for jobs assert. It checks
// both modified fields and ensures that unspecified fields remain unchanged.
func assertBaseConfigJobs(t *testing.T, orig apps.Config, cmd baseConfig, got apps.Config) {
	t.Helper()

	var wantDeployJobs []apps.DeployJob
	if cmd.DeployJob != nil {
		if cmd.DeployJob.Enabled == nil || *cmd.DeployJob.Enabled {
			wantDeployJobs = test.PtrToSlice(cmd.DeployJob, deployJobFromCmd)
		}
	}
	gotDeployJobs := test.PtrToSlice(got.DeployJob, func(d *apps.DeployJob) apps.DeployJob { return *d })
	test.AssertJobsEqual(t, wantDeployJobs, gotDeployJobs, func(d apps.DeployJob) test.DeployJobKey { return test.ToDeployJobKey(&d) })

	wantWorkerJobs := appendPtrSlice(orig.WorkerJobs, cmd.WorkerJob, workerJobFromCmd)
	if cmd.DeleteWorkerJob != nil {
		jobName := *cmd.DeleteWorkerJob
		wantWorkerJobs = removeWorkerJob(wantWorkerJobs, jobName)
	}
	gotWorkerJobs := test.NormalizeSlice(got.WorkerJobs)
	test.AssertJobsEqual(t, wantWorkerJobs, gotWorkerJobs, test.ToWorkerJobKey)

	wantScheduledJobs := appendPtrSlice(orig.ScheduledJobs, cmd.ScheduledJob, scheduledJobFromCmd)
	if cmd.DeleteScheduledJob != nil {
		jobName := *cmd.DeleteScheduledJob
		wantScheduledJobs = removeScheduledJob(wantScheduledJobs, jobName)
	}
	gotScheduledJobs := test.NormalizeSlice(got.ScheduledJobs)
	test.AssertJobsEqual(t, wantScheduledJobs, gotScheduledJobs, test.ToScheduledJobKey)
}

// deployJobFromCmd converts the CLI deployJob into the CRD DeployJob.
func deployJobFromCmd(j *deployJob) apps.DeployJob {
	if j == nil {
		return apps.DeployJob{}
	}
	dj := apps.DeployJob{
		Job: apps.Job{
			Name:    ptr.Deref(j.Name, ""),
			Command: ptr.Deref(j.Command, ""),
		},
	}
	if j.Retries != nil {
		dj.Retries = j.Retries
	}
	if j.Timeout != nil {
		dj.Timeout = &metav1.Duration{Duration: *j.Timeout}
	}
	return dj
}

// workerJobFromCmd converts the CLI workerJob into the CRD WorkerJob.
func workerJobFromCmd(j *workerJob) apps.WorkerJob {
	if j == nil {
		return apps.WorkerJob{}
	}
	return apps.WorkerJob{
		Job: apps.Job{
			Name:    ptr.Deref(j.Name, ""),
			Command: ptr.Deref(j.Command, ""),
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

	out := apps.ScheduledJob{
		Job: apps.Job{
			Name:    ptr.Deref(j.Name, ""),
			Command: ptr.Deref(j.Command, ""),
		},
		Size:     ptr.To(apps.ApplicationSize(ptr.Deref(j.Size, ""))),
		Schedule: ptr.Deref(j.Schedule, ""),
	}
	if j.Retries != nil {
		out.Retries = new(int32)
		*out.Retries = *j.Retries
	}
	if j.Timeout != nil {
		out.Timeout = new(metav1.Duration)
		out.Timeout.Duration = *j.Timeout
	}
	return out
}

// appendPtrSlice takes any existing slice of Ts plus an optional
// *S from the CLI and returns the append.
func appendPtrSlice[S any, T any](orig []T, cli *S, conv func(*S) T) []T {
	// copy orig so we don't clobber it
	out := append([]T(nil), orig...)
	if cli != nil {
		out = append(out, conv(cli))
	}
	return out
}

// removeJobByName returns a new slice containing only those items in orig
// whose name (as extracted by getName) does *not* equal the given name.
// If no items match, it simply returns a copy of orig.
func removeJobByName[T any](
	orig []T,
	name string,
	getName func(T) string,
) []T {
	var out []T
	for _, item := range orig {
		if getName(item) != name {
			out = append(out, item)
		}
	}
	return out
}

func removeWorkerJob(
	orig []apps.WorkerJob,
	name string,
) []apps.WorkerJob {
	return removeJobByName(orig, name, func(w apps.WorkerJob) string {
		return w.Job.Name
	})
}

func removeScheduledJob(
	orig []apps.ScheduledJob,
	name string,
) []apps.ScheduledJob {
	return removeJobByName(orig, name, func(s apps.ScheduledJob) string {
		return s.Job.Name
	})
}
