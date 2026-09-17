package delete

import (
	"context"
	"os"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type fromFile struct {
	format.Writer
	Filename *os.File `short:"f" completion-predictor:"local:file"`
}

func (cmd *fromFile) Run(ctx context.Context, client *api.Client) error {
	obj, err := client.DeleteFromFile(ctx, cmd.Filename)
	if err != nil {
		return err
	}
	cmd.Successf("🗑", "deleted %s", format.Object(obj))

	return nil
}
