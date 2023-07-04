package format

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/fatih/color"
	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/printer"
	"github.com/theckman/yacspin"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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

// SuccessMessagef is a formatted message for indicating a successful step.
func SuccessMessagef(icon, format string, a ...any) string {
	return fmt.Sprintf(" %s %s %s", SuccessChar, fmt.Sprintf(format, a...), icon)
}

// PrintSuccessf prints a success message.
func PrintSuccessf(icon, format string, a ...any) {
	fmt.Print(SuccessMessagef(icon, format, a...) + "\n")
}

// FailureMessagef is a formatted message for indicating a failed step.
func FailureMessagef(icon, format string, a ...any) string {
	return fmt.Sprintf(" %s %s %s", FailureChar, fmt.Sprintf(format, a...), icon)
}

// PrintFailuref prints a failure message.
func PrintFailuref(icon, format string, a ...any) {
	fmt.Print(FailureMessagef(icon, format, a...) + "\n")
}

// Confirmf prints a confirm dialog using format and then waits until prompt
// is confirmed or denied. Only y and yes are accepted for confirmation.
func Confirmf(format string, a ...any) (bool, error) {
	var input string

	fmt.Printf("%s [y|n]: ", fmt.Sprintf(format, a...))
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

// PrettyPrintObjects prints the supplied objects in "pretty" colored yaml
// with some metadata, status and other default fields stripped out. If
// multiple objects are supplied, they will be divided with a yaml divider.
func PrettyPrintObjects[T resource.Managed](objs []T, opts PrintOpts) error {
	for i, obj := range objs {
		strippedObj, err := stripObj(obj, opts.ExcludeAdditional)
		if err != nil {
			return err
		}
		if err := PrettyPrintResource(strippedObj, opts); err != nil {
			return err
		}
		// if there's another object we print a yaml divider
		if i != len(objs)-1 {
			fmt.Println("---")
		}
	}

	return nil
}

// PrettyPrintResource prints the resource similar to how
// https://github.com/goccy/go-yaml#ycat does it.
func PrettyPrintResource(obj interface{}, opts PrintOpts) error {
	b, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}

	p := printer.Printer{
		LineNumber: false,
		Bool: printerProperty(&printer.Property{
			Prefix: format(color.FgHiMagenta),
			Suffix: format(color.Reset),
		}),
		MapKey: printerProperty(&printer.Property{
			Prefix: format(color.FgHiCyan),
			Suffix: format(color.Reset),
		}),
		Number: printerProperty(&printer.Property{
			Prefix: format(color.FgHiMagenta),
			Suffix: format(color.Reset),
		}),
		String: printerProperty(&printer.Property{
			Prefix: format(color.FgHiGreen),
			Suffix: format(color.Reset),
		}),
	}

	output := []byte(p.PrintTokens(lexer.Tokenize(string(b))) + "\n")

	if opts.Out == nil {
		opts.Out = os.Stdout
	}

	_, err = opts.Out.Write(output)
	return err
}

// stripObj removes some fields which simply add clutter to the yaml output.
// The object should still be applyable afterwards as just defaults and
// computed fields get removed.
func stripObj(obj resource.Managed, excludeAdditional [][]string) (map[string]any, error) {
	strippedObj := obj.DeepCopyObject().(resource.Managed)
	strippedObj.SetManagedFields(nil)
	strippedObj.SetResourceVersion("")
	strippedObj.SetUID("")
	strippedObj.SetGeneration(0)
	strippedObj.SetProviderConfigReference(nil)
	strippedObj.SetDeletionPolicy("")
	strippedObj.SetFinalizers(nil)

	annotations := strippedObj.GetAnnotations()
	for k := range annotations {
		if strings.HasPrefix(k, "crossplane.io") ||
			strings.HasPrefix(k, "kubectl.kubernetes.io") {
			delete(annotations, k)
		}
	}
	strippedObj.SetAnnotations(annotations)

	// some fields cannot be removed with a Set, so we convert to unstructured
	// to get rid of these.
	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(strippedObj)
	if err != nil {
		return nil, err
	}

	unstructured.RemoveNestedField(unstructuredObj, "status", "conditions")
	unstructured.RemoveNestedField(unstructuredObj, "metadata", "creationTimestamp")

	for _, exclude := range excludeAdditional {
		unstructured.RemoveNestedField(unstructuredObj, exclude...)
	}

	return unstructuredObj, nil
}

func printerProperty(p *printer.Property) printer.PrintFunc {
	return func() *printer.Property {
		return p
	}
}
