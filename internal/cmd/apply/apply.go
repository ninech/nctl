// Package apply provides the implementation for the apply command,
// allowing users to apply resources from files.
package apply

type Cmd struct {
	FromFile fromFile `cmd:"" default:"withargs" name:"-f <file>" help:"Apply any resource from a yaml or json file."`
}
