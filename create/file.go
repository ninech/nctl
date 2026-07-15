package create

import (
	"context"
	"os"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/apply"
	"github.com/ninech/nctl/internal/format"
)

type fromFile struct {
	format.Writer
	Filename *os.File `short:"f" help:"Create any resource from a yaml or json file." completion-predictor:"file"`
}

func (cmd *fromFile) Run(ctx context.Context, client *api.Client) error {
	return apply.File(ctx, cmd.Writer, client, cmd.Filename)
}
