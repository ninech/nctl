package auth

import (
	"context"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type PrintAccessTokenCmd struct {
	format.Writer
}

func (cmd *PrintAccessTokenCmd) Run(ctx context.Context, client *api.Client) error {
	cmd.Println(client.Token(ctx))
	return nil
}
