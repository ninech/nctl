package format

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kong"
)

const (
	LoginCommand  = "login"
	LogoutCommand = "logout"
)

type command string

// Command can be used to print how certain nctl commands can be executed.
func Command() command {
	return command(os.Args[0])
}

// Login returns the login command.
func (c command) Login() string {
	return fmt.Sprintf("%s %s", string(c), LoginCommand)
}

// Get returns the command for getting a resource with nctl.
func (c command) Get(fields ...string) string {
	for i := range fields {
		fields[i] = strings.ToLower(fields[i])
	}

	return fmt.Sprintf("%s get %s", string(c), strings.Join(fields, " "))
}

// GetProjects returns the command for listing projects.
func (c command) GetProjects() string {
	return fmt.Sprintf("%s get projects", string(c))
}

// WhoAmI returns the command for showing current user info.
func (c command) WhoAmI() string {
	return fmt.Sprintf("%s auth whoami", string(c))
}

// SetOrg returns the command for setting the organization.
func (c command) SetOrg(org string) string {
	if org == "" {
		return fmt.Sprintf("%s auth set-org <org-name>", string(c))
	}
	return fmt.Sprintf("%s auth set-org %s", string(c), org)
}

// SetProject returns the command for setting the project.
func (c command) SetProject(project string) string {
	if project == "" {
		return fmt.Sprintf("%s auth set-project <project-name>", string(c))
	}
	return fmt.Sprintf("%s auth set-project %s", string(c), project)
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
