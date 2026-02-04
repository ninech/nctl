package delete

import (
	"context"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/apply"
	"github.com/ninech/nctl/internal/format"
)

type fromFile struct {
	format.Writer
}

func (cmd *fromFile) Run(ctx context.Context, client *api.Client, delete *Cmd) error {
	return apply.File(ctx, cmd.Writer, client, delete.Filename, apply.Delete())
}
