package completion

import (
	"slices"

	"github.com/alecthomas/kong"
	"github.com/posener/complete"
)

const projectFlag = "project"

// projectFinder reads the project off the command line being completed. It
// holds the names of the project flag, which Kong has not parsed yet when
// completion runs.
type projectFinder []string

// newProjectFinder returns a finder for the project flag of parser. Without
// such a flag it finds nothing, leaving the predictors with the project of the
// kubeconfig.
func newProjectFinder(parser *kong.Kong) projectFinder {
	return projectFinder(flagNames(findFlag(parser, projectFlag)))
}

// find extracts the project from args, returning whether the flag is currently
// incomplete (e.g. a trailing -p or --project).
func (f projectFinder) find(args complete.Args) (string, bool) {
	if len(f) == 0 {
		return "", false
	}

	if slices.Contains(f, args.LastCompleted) {
		return "", true
	}

	if p := flagValue(args.All, f...); p != "" {
		return p, false
	}

	// Fall back to COMP_LINE if args.All was consumed by subcommand matching.
	if p := flagValue(argsFromENV(), f...); p != "" {
		return p, false
	}

	return "", false
}
