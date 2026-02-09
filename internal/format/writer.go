package format

import (
	"fmt"
	"io"
	"strings"

	"github.com/theckman/yacspin"
)

// Writer is a wrapper around an [io.Writer] that provides helper methods for
// printing formatted messages.
type Writer struct {
	io.Writer
}

// NewWriter returns a new [Writer].
func NewWriter(w io.Writer) Writer {
	return Writer{Writer: w}
}

// writer returns the underlying writer, or [io.Discard] if the receiver is nil.
func (w *Writer) writer() io.Writer {
	if w == nil || w.Writer == nil {
		return io.Discard
	}

	return w.Writer
}

// BeforeApply ensures that Kong initializes the writer.
func (w *Writer) BeforeApply(writer io.Writer) error {
	if w != nil && writer != nil {
		w.Writer = writer
	}

	return nil
}

// Spinner creates a new spinner with the given message and stop message.
func (w *Writer) Spinner(message, stopMessage string) (*yacspin.Spinner, error) {
	return newSpinner(w.writer(), message, stopMessage)
}

// Successf is a formatted message for indicating a successful step.
func (w *Writer) Successf(icon string, format string, a ...any) {
	fmt.Fprint(w.writer(), successf(icon, format, a...)+"\n")
}

// Success returns a message for indicating a successful step.
func (w *Writer) Success(icon string, message string) {
	fmt.Fprint(w.writer(), success(icon, message)+"\n")
}

// Warningf is a formatted message for indicating a warning.
func (w *Writer) Warningf(format string, a ...any) {
	fmt.Fprint(w.writer(), warningf(format, a...)+"\n")
}

// Failuref is a formatted message for indicating a failure.
func (w *Writer) Failuref(icon string, format string, a ...any) {
	fmt.Fprint(w.writer(), failuref(icon, format, a...)+"\n")
}

// Infof is a formatted message for providing information.
func (w *Writer) Infof(icon string, format string, a ...any) {
	fmt.Fprint(w.writer(), infof(icon, format, a...)+"\n")
}

// Info returns a message for providing information.
func (w *Writer) Info(icon string, message string) {
	fmt.Fprint(w.writer(), info(icon, message)+"\n")
}

// Printf formats according to a format specifier and writes to the underlying writer.
func (w *Writer) Printf(format string, a ...any) {
	fmt.Fprintf(w.writer(), format, a...)
}

// Println prints a formatted message to the underlying writer.
func (w *Writer) Println(a ...any) {
	fmt.Fprintln(w.writer(), a...)
}

// Confirm prints a confirm dialog using the supplied message and then waits
// until prompt is confirmed or denied. Only y and yes are accepted for
// confirmation.
func (w *Writer) Confirm(reader Reader, message string) (bool, error) {
	var input string

	w.Printf("%s [y|n]: ", message)
	_, err := fmt.Fscanln(reader, &input)
	if err != nil {
		return false, err
	}
	input = strings.ToLower(input)

	if input == "y" || input == "yes" {
		return true, nil
	}
	return false, nil
}
