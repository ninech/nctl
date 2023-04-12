package create

import (
	"context"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/apply"
)

type fromFile struct {
}

func (cmd *fromFile) Run(ctx context.Context, client *api.Client, create *Cmd) error {
	return apply.File(ctx, client, create.Filename)
}
