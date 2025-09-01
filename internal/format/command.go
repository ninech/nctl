package format

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kong"
)

const (
	LoginCommand          = "login"
	LogoutCommand         = "logout"
	getApplicationCommand = "get application"
)

type command string

// Command can be used to print how certain nctl commands can be executed
func Command() command {
	return command(os.Args[0])
}

// Login returns the login command
func (c command) Login() string {
	return fmt.Sprintf("%s %s", string(c), LoginCommand)
}

// GetApplication returns the command for getting applications with nctl
func (c command) GetApplication(extraFields ...string) string {
	if len(extraFields) == 0 {
		return fmt.Sprintf("%s %s", string(c), getApplicationCommand)
	}
	return fmt.Sprintf("%s %s %s", string(c), getApplicationCommand, strings.Join(extraFields, " "))
}

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
func ExitIfErrorf(err error, args ...any) error {
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

	fmt.Printf("\n💡 Your command: %q: %s\n", command, msg)

	parseErr.Context.Exit(1)

	return nil
}
