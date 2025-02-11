package logs

import (
	"context"
	"errors"
	"fmt"
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
	if cmd.Name != "" {
		build := &apps.Build{}
		if err := client.GetObject(ctx, cmd.Name, build); err != nil {
			return err
		}
		if time.Since(build.CreationTimestamp.Time) > logRetention {
			return fmt.Errorf(
				"the logs of the build %s are not available as the build is more than %.f days old",
				build.Name, logRetention.Hours()/24,
			)
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
