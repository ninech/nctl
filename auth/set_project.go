package auth

import (
	"context"

	"github.com/ninech/nctl/api"
)

type SetProjectCmd struct {
	Name string `arg:"" predictor:"resource_name" help:"Name of the default project to be used."`
}

func (s *SetProjectCmd) Run(ctx context.Context, client *api.Client) error {
	return SetContextProject(client.KubeconfigPath, client.KubeconfigContext, s.Name)
}
