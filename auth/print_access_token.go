package auth

import (
	"context"
	"fmt"

	"github.com/ninech/nctl/api"
)

type PrintAccessTokenCmd struct{}

func (o *PrintAccessTokenCmd) Run(ctx context.Context, client *api.Client) error {
	fmt.Println(client.Token)
	return nil
}
