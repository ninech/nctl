package apply

import "os"

type Cmd struct {
	Filename *os.File `short:"f" predictor:"file"`
	FromFile fromFile `cmd:"" default:"1" name:"-f <file>" help:"Apply any resource from a yaml or json file."`
}
