package logs

import (
	"context"

	"github.com/ninech/nctl/api"
)

type buildCmd struct {
	Name string `arg:"" help:"Name of the Build."`
	logsCmd
}

func (cmd *buildCmd) Run(ctx context.Context, client *api.Client) error {
	return cmd.logsCmd.Run(ctx, client, BuildQuery(cmd.Name, client.Namespace))
}

const buildLabel = "build"

func BuildQuery(name, namespace string) string {
	return queryString(buildLabel, name, namespace)
}
