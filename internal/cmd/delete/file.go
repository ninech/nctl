package delete

import (
	"context"
	"os"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type fromFile struct {
	format.Writer
	Filename *os.File `short:"f" required:"" completion-predictor:"local:file"`
}

func (cmd *fromFile) Run(ctx context.Context, client *api.Client) error {
	defer cmd.Filename.Close()

	obj, err := client.DeleteManifest(ctx, cmd.Filename)
	if err != nil {
		return err
	}
	cmd.Successf("🗑", "deleted %s", format.Object(obj))

	return nil
}
