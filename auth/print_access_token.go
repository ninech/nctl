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
	token, err := client.TokenE(ctx)
	if err != nil {
		return err
	}

	cmd.Println(token)
	return nil
}
