package apply

import (
	"context"
	"os"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type fromFile struct {
	format.Writer `hidden:""`
	Filename      *os.File `short:"f" required:"" completion-predictor:"local:file"`
}

func (cmd *fromFile) Run(ctx context.Context, client *api.Client) error {
	defer cmd.Filename.Close()

	obj, result, err := client.ApplyManifest(ctx, cmd.Filename)
	if err != nil {
		return err
	}

	verb := "created"
	if result == api.ApplyResultUpdated {
		verb = "applied"
	}
	cmd.Successf("🏗", "%s %s", verb, format.Object(obj))

	return nil
}
