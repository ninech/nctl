package logs

import (
	"context"
	"errors"
	"time"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
)

type buildCmd struct {
	resourceCmd
	logsCmd
	ApplicationName string `short:"a" help:"Name of the application to get build logs for."`
}

func (cmd *buildCmd) Run(ctx context.Context, client *api.Client) error {
	if cmd.Name == "" && cmd.ApplicationName == "" {
		return errors.New("please specify a build name or an application name to see build logs from")
	}
	build := &apps.Build{}
	if cmd.Name != "" {
		if err := client.Get(ctx, api.NamespacedName(cmd.Name, client.Project), build); err != nil {
			return err
		}
		cmd.Since = time.Since(build.CreationTimestamp.Time)
	}

	query := BuildQuery(cmd.Name, client.Project)
	if len(cmd.ApplicationName) != 0 {
		query = BuildsOfAppQuery(cmd.ApplicationName, client.Project)
	}

	return cmd.logsCmd.Run(ctx, client, query, apps.LogLabelBuild)
}

func BuildQuery(name, project string) string {
	return buildQuery(inProject(project), queryExpr(opEquals, apps.LogLabelBuild, name))
}

func BuildsOfAppQuery(name, project string) string {
	return buildQuery(
		inProject(project),
		queryExpr(opEquals, apps.LogLabelApplication, name),
		queryExpr(opNotEquals, apps.LogLabelBuild, ""),
	)
}
