package create

import (
	"context"
	"os"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type fromFile struct {
	format.Writer
	Filename *os.File `short:"f" required:"" help:"Create any resource from a yaml or json file." completion-predictor:"local:file"`
}

func (cmd *fromFile) Run(ctx context.Context, client *api.Client) error {
	defer cmd.Filename.Close()

	obj, err := client.CreateManifest(ctx, cmd.Filename)
	if err != nil {
		return err
	}
	cmd.Successf("🏗", "created %s", format.Object(obj))

	return nil
}
