package logs

import (
	"context"
	"errors"
	"time"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
)

type buildCmd struct {
	Name            string `arg:"" default:"" help:"Name of the Build."`
	ApplicationName string `short:"a" help:"Name of the application to get build logs for."`
	logsCmd
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
		query = queryString(map[string]string{
			appLabel:   cmd.ApplicationName,
			phaseLabel: buildPhase,
		}, client.Project)
	}

	return cmd.logsCmd.Run(ctx, client, query)
}

const (
	buildLabel = "build"
	buildPhase = "build"
)

func BuildQuery(name, project string) string {
	return queryString(map[string]string{buildLabel: name, phaseLabel: buildPhase}, project)
}

func BuildsOfAppQuery(name, project string) string {
	return queryString(map[string]string{appLabel: name, phaseLabel: buildPhase}, project)
}
