package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/alecthomas/kong"
)

// missingCommand reports whether parsing failed
// only because a command or a required argument is missing.
func missingCommand(parseErr *kong.ParseError) bool {
	if parseErr.Context.Error != nil {
		return false
	}

	node := parseErr.Context.Selected()
	if node == nil {
		node = parseErr.Context.Model.Node
	}

	return missingChildren(node)
}

// missingChildren detects missing commands/args.
// Logic taken from github.com/alecthomas/kong/context.go
func missingChildren(node *kong.Node) bool {
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

// printMissingCommand prints the usage of the command the user typed
// and a friendly message about what is missing to stderr.
// Like Kong on a parse error, it prints the usage to the parser's stdout.
func printMissingCommand(stderr io.Writer, parseErr *kong.ParseError) error {
	if err := parseErr.Context.PrintUsage(false); err != nil {
		return err
	}

	command := strings.TrimSpace(parseErr.Context.Model.Name + " " + parseErr.Context.Command())
	_, err := fmt.Fprintf(stderr, "\n💡 Your command: %q: %s\n", command, parseErr.Error())

	return err
}
