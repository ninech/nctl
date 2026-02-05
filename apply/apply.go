// Package apply provides the implementation for the apply command,
// allowing users to apply resources from files.
package apply

import (
	"os"

	"github.com/ninech/nctl/internal/format"
)

type Cmd struct {
	format.Writer `kong:"-"`
	Filename      *os.File `short:"f" completion-predictor:"file"`
	FromFile      fromFile `cmd:"" default:"1" name:"-f <file>" help:"Apply any resource from a yaml or json file."`
}
