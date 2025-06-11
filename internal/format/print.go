package format

import (
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

const (
	SuccessChar      = "✓"
	FailureChar      = "✗"
	spinnerPrefix    = " "
	spinnerFrequency = 100 * time.Millisecond
)

var spinnerCharset = yacspin.CharSets[24]

// ProgressMessagef is a formatted message for use with a spinner.Suffix. An
// icon can be added which is displayed at the end of the message.
func ProgressMessagef(icon, format string, a ...any) string {
	return fmt.Sprintf(" %s %s", fmt.Sprintf(format, a...), icon)
}

// ProgressMessage is a formatted message for use with a spinner.Suffix. An
// icon can be added which is displayed at the end of the message.
func ProgressMessage(icon, message string) string {
	return fmt.Sprintf(" %s %s", message, icon)
}

// SuccessMessagef is a formatted message for indicating a successful step.
func SuccessMessagef(icon, format string, a ...any) string {
	return fmt.Sprintf(" %s %s %s", SuccessChar, fmt.Sprintf(format, a...), icon)
}

// SuccessMessage returns a message for indicating a successful step.
func SuccessMessage(icon, message string) string {
	return fmt.Sprintf(" %s %s %s", SuccessChar, message, icon)
}

// PrintSuccessf prints a success message.
func PrintSuccessf(icon, format string, a ...any) {
	fmt.Print(SuccessMessagef(icon, format, a...) + "\n")
}

// PrintSuccess prints a success message.
func PrintSuccess(icon, message string) {
	fmt.Print(SuccessMessage(icon, message) + "\n")
}

// FailureMessagef is a formatted message for indicating a failed step.
func FailureMessagef(icon, format string, a ...any) string {
	return fmt.Sprintf(" %s %s %s", FailureChar, fmt.Sprintf(format, a...), icon)
}

// PrintFailuref prints a failure message.
func PrintFailuref(icon, format string, a ...any) {
	fmt.Print(FailureMessagef(icon, format, a...) + "\n")
}

func PrintWarningf(msg string, a ...any) {
	fmt.Printf(color.YellowString("Warning: ")+msg, a...)
}

// Confirm prints a confirm dialog using the supplied message and then waits
// until prompt is confirmed or denied. Only y and yes are accepted for
// confirmation.
func Confirm(message string) (bool, error) {
	var input string

	fmt.Printf("%s [y|n]: ", message)
	_, err := fmt.Scanln(&input)
	if err != nil {
		return false, err
	}
	input = strings.ToLower(input)

	if input == "y" || input == "yes" {
		return true, nil
	}
	return false, nil
}

// Confirmf prints a confirm dialog using format and then waits until prompt
// is confirmed or denied. Only y and yes are accepted for confirmation.
func Confirmf(format string, a ...any) (bool, error) {
	return Confirm(fmt.Sprintf(format, a...))
}

// NewSpinner returns a new spinner with the default config
func NewSpinner(message, stopMessage string) (*yacspin.Spinner, error) {
	return yacspin.New(spinnerConfig(message, stopMessage))
}

func spinnerConfig(message, stopMessage string) yacspin.Config {
	return yacspin.Config{
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
	for i, obj := range objs {
		if err := PrettyPrintObject(obj, opts); err != nil {
			return err
		}
		// if there's another object we print a yaml divider
		if i != len(objs)-1 {
			fmt.Fprintln(opts.defaultOut(), "---")
		}
	}

	return nil
}

// PrettyPrintObject prints the supplied object in "pretty" colored yaml
// with some metadata, status and other default fields stripped out.
func PrettyPrintObject(obj any, opts PrintOpts) error {
	// we check if we can make a copy of the object as we might alter it.
	// If we can't make a copy of the object we print it directly as
	// altering it would change the source object
	runtimeObject, is := obj.(runtime.Object)
	if !is {
		return printResource(obj, opts)
	}
	objCopy := runtimeObject.DeepCopyObject()

	var toPrint interface{} = objCopy
	if res, is := objCopy.(resource.Object); is {
		var err error
		toPrint, err = stripObj(res, opts.ExcludeAdditional)
		if err != nil {
			return err
		}
	}
	// if we got an unstructured object passed we want to remove the
	// 'object' key from the yaml
	if u, is := toPrint.(*unstructured.Unstructured); is {
		toPrint = u.Object
	}
	return printResource(toPrint, opts)
}

// printResource prints the resource similar to how
// https://github.com/goccy/go-yaml#ycat does it.
func printResource(obj any, opts PrintOpts) error {
	b, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}

	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	p, err := getPrinter(opts.Out)
	if err != nil {
		return err
	}

	output := []byte(p.PrintTokens(lexer.Tokenize(string(b))) + "\n")
	_, err = opts.Out.Write(output)
	return err
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
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj, obj); err != nil {
		return nil, err
	}

	return obj, nil
}

func printerProperty(p *printer.Property) printer.PrintFunc {
	return func() *printer.Property {
		return p
	}
}
