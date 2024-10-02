package auth

import (
	"context"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/config"
)

type SetProjectCmd struct {
	Name string `arg:"" predictor:"resource_name" help:"Name of the default project to be used."`
}

func (s *SetProjectCmd) Run(ctx context.Context, client *api.Client) error {
	return config.SetContextProject(client.KubeconfigPath, client.KubeconfigContext, s.Name)
}
