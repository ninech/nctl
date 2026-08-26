package apifield

import (
	"errors"
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
)

const (
	completionPredictorTag = "completion-predictor"

	helpFormat = "Available values: %s."
)

// Apply returns a kong.PostBuild hook that enriches flags with known API field
// information, appending available values to the help text and setting default placeholders.
func Apply() func(*kong.Kong) error {
	return func(k *kong.Kong) error {
		if k.Model == nil {
			return errors.New("no kong model found")
		}

		return kong.Visit(k.Model.Node, func(node kong.Visitable, next kong.Next) error {
			value, ok := node.(*kong.Value)
			if !ok || value.Tag == nil {
				return next(nil)
			}

			name, ok := unqualify(value.Tag.Get(completionPredictorTag))
			if !ok {
				return next(nil)
			}
			f, ok := fields[name]
			if !ok {
				return next(fmt.Errorf("flag %q names the unknown API field %q", value.Name, name))
			}
			if len(f.Values) == 0 && f.Default == "" {
				return next(fmt.Errorf("field %q of flag %q is empty", name, value.Name))
			}

			if len(f.Values) != 0 {
				value.Help = appendSentence(value.Help, fmt.Sprintf(helpFormat, strings.Join(f.Values, ", ")))
			}
			// Do not overwrite explicitly defined placeholders.
			if f.Default != "" && value.Flag != nil && value.Flag.PlaceHolder == "" {
				value.Flag.PlaceHolder = f.Default
			}

			return next(nil)
		})
	}
}

// appendSentence appends s to help, ensuring proper spacing and punctuation.
func appendSentence(help, s string) string {
	help = strings.TrimRight(help, " ")
	switch {
	case help == "":
		return s
	case strings.HasSuffix(help, "."), strings.HasSuffix(help, "!"), strings.HasSuffix(help, "?"):
		return help + " " + s
	default:
		return help + ". " + s
	}
}
