package logs

import (
	"context"
	"errors"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
)

type applicationCmd struct {
	resourceCmd
	logsCmd
	Type appLogType `short:"t" help:"Which type of app logs to output. ${enum}" enum:"all,app,build,worker_job,deploy_job" default:"all"`
}

func (cmd *applicationCmd) Run(ctx context.Context, client *api.Client) error {
	if cmd.Name == "" {
		return errors.New("please specify an application name")
	}

	app := &apps.Application{}
	if err := client.Get(ctx, api.NamespacedName(cmd.Name, client.Project), app); err != nil {
		return err
	}

	return cmd.logsCmd.Run(ctx, client, buildQuery(append(
		cmd.Type.queryExpressions(),
		inProject(client.Project),
		queryExpr(opEquals, apps.LogLabelApplication, app.Name))...),
		apps.LogLabelBuild, apps.LogLabelReplica, apps.LogLabelWorkerJob, apps.LogLabelDeployJob,
	)
}

func ApplicationQuery(name, project string) string {
	return buildQuery(
		inProject(project),
		queryExpr(opEquals, apps.LogLabelApplication, name),
		queryExpr(opEquals, apps.LogLabelBuild, ""),
	)
}

type appLogType string

const (
	logTypeAll       appLogType = "all"
	logTypeApp       appLogType = "app"
	logTypeBuild     appLogType = "build"
	logTypeDeployJob appLogType = "deploy_job"
	logTypeWorkerJob appLogType = "worker_job"
)

func (a appLogType) queryExpressions() []string {
	expr := []string{}
	switch a {
	case logTypeAll:
		return expr
	case logTypeApp:
		expr = append(expr,
			queryExpr(opEquals, apps.LogLabelDeployJob, ""),
			queryExpr(opEquals, apps.LogLabelWorkerJob, ""),
			queryExpr(opEquals, apps.LogLabelBuild, ""))
	case logTypeBuild:
		expr = append(expr, queryExpr(opNotEquals, apps.LogLabelBuild, ""))
	case logTypeDeployJob:
		expr = append(expr, queryExpr(opNotEquals, apps.LogLabelDeployJob, ""))
	case logTypeWorkerJob:
		expr = append(expr, queryExpr(opNotEquals, apps.LogLabelWorkerJob, ""))
	}
	return expr
}
