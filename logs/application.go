package logs

import (
	"context"

	"github.com/ninech/nctl/api"
)

type applicationCmd struct {
	Name string `arg:"" help:"Name of the Application."`
	logsCmd
}

func (cmd *applicationCmd) Run(ctx context.Context, client *api.Client) error {
	return cmd.logsCmd.Run(ctx, client, ApplicationQuery(cmd.Name, client.Project))
}

const (
	appLabel     = "app"
	runtimePhase = "runtime"
)

func ApplicationQuery(name, project string) string {
	return queryString(map[string]string{appLabel: name, phaseLabel: runtimePhase}, project)
}
