package apply

type Cmd struct {
	Filename string   `short:"f" predictor:"file"`
	FromFile fromFile `cmd:"" default:"1" name:"-f <file>" help:"Apply any resource from a yaml or json file."`
}
