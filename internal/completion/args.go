package completion

import (
	"os"
	"slices"
	"strings"

	"github.com/alecthomas/kong"
)

// argsFromENV parses COMP_LINE as a fallback when posener/complete consumes args.All.
func argsFromENV() []string {
	if line := os.Getenv("COMP_LINE"); line != "" {
		return strings.Fields(line)
	}

	return nil
}

// flagValue returns the value of the first flag in args matching any of names,
// in either the "--flag value" or the "--flag=value" form. It returns an empty
// string if no such flag carries a value.
func flagValue(args []string, names ...string) string {
	for i, arg := range args {
		name, value, assigned := strings.Cut(arg, "=")
		if !slices.Contains(names, name) {
			continue
		}
		if assigned {
			return value
		}
		if i+1 < len(args) {
			return args[i+1]
		}
	}

	return ""
}

// flagNames returns every name under which flag can be given on the command
// line, including its shorthand and aliases.
func flagNames(flag *kong.Flag) []string {
	if flag == nil {
		return nil
	}

	names := make([]string, 0, 2+len(flag.Aliases))
	names = append(names, "--"+flag.Name)
	if flag.Short != 0 {
		names = append(names, "-"+string(flag.Short))
	}
	for _, alias := range flag.Aliases {
		names = append(names, "--"+alias)
	}

	return names
}

// findFlag returns the application-level flag with the given name, or nil if
// the parser does not define it.
func findFlag(parser *kong.Kong, name string) *kong.Flag {
	if parser == nil || parser.Model == nil {
		return nil
	}

	for _, flag := range parser.Model.Flags {
		if flag != nil && flag.Name == name {
			return flag
		}
	}

	return nil
}
