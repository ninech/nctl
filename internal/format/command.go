package format

import (
	"errors"
	"fmt"
	"io"

	"github.com/alecthomas/kong"
)

// MissingChildren detects missing commands/args.
// Logic taken from github.com/alecthomas/kong/context.go
func MissingChildren(node *kong.Node) bool {
	for _, arg := range node.Positional {
		if arg.Required && !arg.Set {
			return true
		}
	}

	for _, child := range node.Children {
		if child.Hidden {
			continue
		}

		if child.Argument != nil {
			if !child.Argument.Required {
				continue
			}
		}

		return true
	}

	return false
}

// ExitIfErrorf prints Usage + friendly message on error (and exits).
func ExitIfErrorf(w io.Writer, err error, args ...any) error {
	if err == nil {
		return nil
	}

	msg := err.Error()

	var parseErr *kong.ParseError
	if errors.As(err, &parseErr) {
		if err := parseErr.Context.PrintUsage(false); err != nil {
			return err
		}
	}

	command := parseErr.Context.Model.Name
	if len(args) > 0 {
		commandArgs := fmt.Sprintf(args[0].(string), args[1:]...)
		if len(commandArgs) > 0 {
			command += " " + commandArgs
		}
	}

	fmt.Fprintf(w, "\n💡 Your command: %q: %s\n", command, msg)

	parseErr.Context.Exit(1)

	return nil
}
