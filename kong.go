package main

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/alecthomas/kong"
)

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

// exitIfErrorf prints Usage + friendly message on error (and exits).
func exitIfErrorf(w io.Writer, err error, args ...any) error {
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

// interpolationRegex is the regex to find if a given string contains variables for interpolation
var interpolationRegex = regexp.MustCompile(`(\$\$)|((?:\${([[:alpha:]_][[:word:]]*))(?:=([^}]+))?})|(\$)|([^$]+)`)

// interpolate interpolates the given string s with variables from vars
// The [upstream function](https://github.com/alecthomas/kong/blob/v0.8.0/interpolate.go#L22)
// was sadly not exported, so we had to copy it.
func interpolate(s string, vars kong.Vars) (string, error) {
	var out strings.Builder
	matches := interpolationRegex.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return s, nil
	}
	for _, match := range matches {
		if dollar := match[1]; dollar != "" {
			out.WriteString("$")
		} else if name := match[3]; name != "" {
			value, ok := vars[name]
			if !ok {
				// No default value.
				if match[4] == "" {
					return "", fmt.Errorf("undefined variable ${%s}", name)
				}
				value = match[4]
			}
			out.WriteString(value)
		} else {
			out.WriteString(match[0])
		}
	}
	return out.String(), nil
}

// interpolateFlagPlaceholders will return a function
// which walks the whole kong model and interpolates variables in placeholders in flags.
func interpolateFlagPlaceholders(vars kong.Vars) func(*kong.Kong) error {
	var walkNode func(n *kong.Node) error
	walkNode = func(n *kong.Node) error {
		var err error
		if n == nil {
			return nil
		}
		for i := range n.Flags {
			if n.Flags[i] == nil {
				continue
			}
			if n.Flags[i].PlaceHolder, err = interpolate(n.Flags[i].PlaceHolder, vars); err != nil {
				return fmt.Errorf("error when interpolating placeholder tag of flag %q: %w", n.Flags[i].Name, err)
			}
		}
		// we are now calling ourselves for all child nodes
		for i := range n.Children {
			if err := walkNode(n.Children[i]); err != nil {
				return err
			}
		}
		return nil
	}
	return func(k *kong.Kong) error {
		if k.Model == nil {
			return errors.New("no kong model found")
		}
		return walkNode(k.Model.Node)
	}
}
