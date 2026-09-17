package logs

import (
	"context"
	"errors"
	"fmt"
	"time"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/log"
)

type buildCmd struct {
	resourceCmd
	LogsCmd
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

	query := log.BuildQuery(cmd.Name, client.Project)
	if len(cmd.ApplicationName) != 0 {
		query = log.BuildsOfAppQuery(cmd.ApplicationName, client.Project)
	}

	return cmd.LogsCmd.Run(ctx, client, query, apps.LogLabelBuild)
}
