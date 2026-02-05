package format

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/alecthomas/kong"
)

// interpolationRegex is the regex to find if a given string contains variables
// for interpolation
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

// InterpolateFlagPlaceholders will return a function which walks the whole kong
// model and interpolates variables in placeholders in flags.
func InterpolateFlagPlaceholders(vars kong.Vars) func(*kong.Kong) error {
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
