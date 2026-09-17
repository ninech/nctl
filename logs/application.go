package logs

import (
	"context"
	"errors"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/log"
)

type applicationCmd struct {
	resourceCmd
	LogsCmd
	Type appLogType `short:"t" help:"Which type of app logs to output. ${enum}" enum:"all,app,build,worker_job,deploy_job,scheduled_job" default:"all"`
}

func (cmd *applicationCmd) Run(ctx context.Context, client *api.Client) error {
	if cmd.Name == "" {
		return errors.New("please specify an application name")
	}
	if err := client.GetObject(ctx, cmd.Name, &apps.Application{}); err != nil {
		return err
	}

	return cmd.LogsCmd.Run(ctx, client, log.Selector(append(
		cmd.Type.matchers(),
		log.InProject(client.Project),
		log.Equal(apps.LogLabelApplication, cmd.Name))...),
		apps.LogLabelBuild, apps.LogLabelReplica, apps.LogLabelWorkerJob, apps.LogLabelDeployJob, apps.LogLabelDeployJob,
	)
}

type appLogType string

const (
	logTypeAll          appLogType = "all"
	logTypeApp          appLogType = "app"
	logTypeBuild        appLogType = "build"
	logTypeDeployJob    appLogType = "deploy_job"
	logTypeWorkerJob    appLogType = "worker_job"
	logTypeScheduledJob appLogType = "scheduled_job"
)

// matchers returns the label matchers narrowing application logs down to the
// log type.
func (a appLogType) matchers() []log.Matcher {
	switch a {
	case logTypeAll:
		return nil
	case logTypeApp:
		return []log.Matcher{
			log.Equal(apps.LogLabelDeployJob, ""),
			log.Equal(apps.LogLabelWorkerJob, ""),
			log.Equal(apps.LogLabelScheduledJob, ""),
			log.Equal(apps.LogLabelBuild, ""),
		}
	case logTypeBuild:
		return []log.Matcher{log.NotEqual(apps.LogLabelBuild, "")}
	case logTypeDeployJob:
		return []log.Matcher{log.NotEqual(apps.LogLabelDeployJob, "")}
	case logTypeWorkerJob:
		return []log.Matcher{log.NotEqual(apps.LogLabelWorkerJob, "")}
	case logTypeScheduledJob:
		return []log.Matcher{log.NotEqual(apps.LogLabelScheduledJob, "")}
	}
	return nil
}
