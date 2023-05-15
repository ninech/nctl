package format

import (
	"fmt"
	"strings"
	"time"

	"github.com/theckman/yacspin"
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
