// Package format contains utilities for formatting and printing output,
// including support for spinners, colored YAML/JSON, and resource stripping.
package format

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/fatih/color"
	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/printer"
	"github.com/mattn/go-isatty"
	"github.com/theckman/yacspin"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

type OutputFormatType int

const (
	SuccessChar      = "✓"
	FailureChar      = "✗"
	spinnerPrefix    = " "
	spinnerFrequency = 100 * time.Millisecond

	OutputFormatTypeYAML OutputFormatType = 0
	OutputFormatTypeJSON OutputFormatType = 1
)

type JSONOutputOptions struct {
	// PrintSingleItem will print a single item of an array as is
	// (without the array notation)
	PrintSingleItem bool
}

var spinnerCharset = yacspin.CharSets[24]

// Progress is a formatted message for use with a spinner.Suffix. An
// icon can be added which is displayed at the end of the message.
func Progress(icon, message string) string {
	return fmt.Sprintf(" %s %s", message, icon)
}

// Progressf is a formatted message for use with a spinner.Suffix. An
// icon can be added which is displayed at the end of the message.
func Progressf(icon, format string, a ...any) string {
	return fmt.Sprintf(" %s %s", fmt.Sprintf(format, a...), icon)
}

// successf is a formatted message for indicating a successful step.
func successf(icon, format string, a ...any) string {
	return fmt.Sprintf(" %s %s %s", SuccessChar, fmt.Sprintf(format, a...), icon)
}

// success returns a message for indicating a successful step.
func success(icon, message string) string {
	return fmt.Sprintf(" %s %s %s", SuccessChar, message, icon)
}

// failuref is a formatted message for indicating a failed step.
func failuref(icon, format string, a ...any) string {
	return fmt.Sprintf(" %s %s %s", FailureChar, fmt.Sprintf(format, a...), icon)
}

func warningf(msg string, a ...any) string {
	return fmt.Sprintf(color.YellowString("Warning: ")+msg, a...)
}

// NewSpinner returns a new spinner with the default config
func newSpinner(w io.Writer, message, stopMessage string) (*yacspin.Spinner, error) {
	return yacspin.New(spinnerConfig(w, message, stopMessage))
}

func spinnerConfig(w io.Writer, message, stopMessage string) yacspin.Config {
	return yacspin.Config{
		Writer:            w,
		Frequency:         spinnerFrequency,
		CharSet:           spinnerCharset,
		Prefix:            spinnerPrefix,
		Message:           message,
		StopMessage:       stopMessage,
		StopFailMessage:   message,
		StopCharacter:     SuccessChar,
		StopFailCharacter: FailureChar,
	}
}

const escape = "\x1b"

func format(attr color.Attribute) string {
	return fmt.Sprintf("%s[%dm", escape, attr)
}

// PrintOpts customizes the printing.
type PrintOpts struct {
	// Out will be used to print to if set instead of stdout.
	Out io.Writer
	// ExcludeAdditional allows to exclude more fields of the object
	ExcludeAdditional [][]string
	// format type of the output, e.g. yaml or json
	Format OutputFormatType
	// JSONOpts defines special options for JSON output
	JSONOpts JSONOutputOptions
	// AllFields prints all fields of the object.
	AllFields bool
}

func (p PrintOpts) defaultOut() io.Writer {
	if p.Out == nil {
		return os.Stdout
	}
	return p.Out
}

// PrettyPrintObjects prints the supplied objects in "pretty" colored yaml
// with some metadata, status and other default fields stripped out. If
// multiple objects are supplied, they will be divided with a yaml divider.
func PrettyPrintObjects[T any](objs []T, opts PrintOpts) error {
	switch opts.Format {
	case OutputFormatTypeJSON:
		// check if we should strip the array around a single item
		if opts.JSONOpts.PrintSingleItem && len(objs) == 1 {
			prepared, err := prepareObject(objs[0], opts)
			if err != nil {
				return err
			}
			return printResource(prepared, opts)

		}
		var toPrint []any
		for _, item := range objs {
			prepared, err := prepareObject(item, opts)
			if err != nil {
				return err
			}
			toPrint = append(toPrint, prepared)
		}
		if err := printResource(toPrint, opts); err != nil {
			return err
		}
	default:
		for i, obj := range objs {
			if err := PrettyPrintObject(obj, opts); err != nil {
				return err
			}
			// if there's another object we print a yaml divider
			if i != len(objs)-1 {
				fmt.Fprintln(opts.defaultOut(), "---")
			}
		}
	}
	return nil
}

func prepareObject(obj any, opts PrintOpts) (any, error) {
	if opts.AllFields {
		return obj, nil
	}
	runtimeObject, is := obj.(runtime.Object)
	if !is {
		return obj, nil
	}
	objCopy := runtimeObject.DeepCopyObject()

	var toPrint any = objCopy
	if res, is := objCopy.(resource.Object); is {
		var err error
		toPrint, err = stripObj(res, opts.ExcludeAdditional)
		if err != nil {
			return nil, err
		}
	}
	// if we got an unstructured object passed we want to remove the
	// 'object' key from the yaml
	if u, is := toPrint.(*unstructured.Unstructured); is {
		toPrint = u.Object
	}
	return toPrint, nil
}

// PrettyPrintObject prints the supplied object in "pretty" colored yaml
// with some metadata, status and other default fields stripped out.
func PrettyPrintObject(obj any, opts PrintOpts) error {
	// we check if we can make a copy of the object as we might alter it.
	// If we can't make a copy of the object we print it directly as
	// altering it would change the source object
	prepared, err := prepareObject(obj, opts)
	if err != nil {
		return nil
	}
	return printResource(prepared, opts)
}

// printResource prints the resource similar to how
// https://github.com/goccy/go-yaml#ycat does it.
func printResource(obj any, opts PrintOpts) error {
	var b []byte
	var err error

	switch opts.Format {
	case OutputFormatTypeJSON:
		b, err = json.MarshalIndent(obj, "", "  ")
		b = append(b, '\n')
	default:
		b, err = yaml.Marshal(obj)
	}

	if err != nil {
		return err
	}

	if opts.Out == nil {
		opts.Out = os.Stdout
	}

	switch opts.Format {
	case OutputFormatTypeJSON:
		_, err = opts.Out.Write(b)
		return err
	default:
		p, err := getPrinter(opts.Out)
		if err != nil {
			return err
		}
		output := []byte(p.PrintTokens(lexer.Tokenize(string(b))) + "\n")
		_, err = opts.Out.Write(output)
		return err
	}
}

// getPrinter returns a printer for printing tokens. It will have color output
// if the given io.Writer is a terminal.
func getPrinter(out io.Writer) (printer.Printer, error) {
	p := printer.Printer{
		LineNumber: false,
	}
	if IsInteractiveEnvironment(out) {
		p.Bool = printerProperty(&printer.Property{
			Prefix: format(color.FgHiMagenta),
			Suffix: format(color.Reset),
		})
		p.MapKey = printerProperty(&printer.Property{
			Prefix: format(color.FgHiCyan),
			Suffix: format(color.Reset),
		})
		p.Number = printerProperty(&printer.Property{
			Prefix: format(color.FgHiMagenta),
			Suffix: format(color.Reset),
		})
		p.String = printerProperty(&printer.Property{
			Prefix: format(color.FgHiGreen),
			Suffix: format(color.Reset),
		})
	}
	return p, nil
}

func IsInteractiveEnvironment(out io.Writer) bool {
	f, isFile := out.(*os.File)
	if !isFile {
		return false
	}
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// stripObj removes some fields which simply add clutter to the yaml output.
// The object should still be applicable afterwards as just defaults and
// computed fields get removed.
func stripObj(obj resource.Object, excludeAdditional [][]string) (resource.Object, error) {
	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")
	obj.SetUID("")
	obj.SetGeneration(0)
	obj.SetFinalizers(nil)

	annotations := obj.GetAnnotations()
	for k := range annotations {
		if strings.HasPrefix(k, "crossplane.io") ||
			strings.HasPrefix(k, "kubectl.kubernetes.io") {
			delete(annotations, k)
		}
	}
	obj.SetAnnotations(annotations)
	if len(obj.GetAnnotations()) == 0 {
		obj.SetAnnotations(nil)
	}

	// some fields cannot be removed with a Set, so we convert to unstructured
	// to get rid of these.
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}

	unstructured.RemoveNestedField(unstructuredObj, "spec", "deletionPolicy")
	unstructured.RemoveNestedField(unstructuredObj, "spec", "providerConfigRef")
	unstructured.RemoveNestedField(unstructuredObj, "spec", "managementPolicies")

	for _, exclude := range excludeAdditional {
		unstructured.RemoveNestedField(unstructuredObj, exclude...)
	}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		unstructuredObj,
		obj,
	); err != nil {
		return nil, err
	}

	return obj, nil
}

func printerProperty(p *printer.Property) printer.PrintFunc {
	return func() *printer.Property {
		return p
	}
}
