package format

import "io"

// IOStreams are the standard input, output and error of a command.
//
// Kong binds them for the BeforeApply hooks of [Writer] and [Reader],
// so that every command reads and writes through the streams passed to the CLI
// instead of the ones of the process.
//
// This is built after [k8s.io/cli-runtime/pkg/genericiooptions.IOStreams].
type IOStreams struct {
	// In is the input commands prompt from.
	In io.Reader
	// Out receives the output of a command.
	Out io.Writer
	// ErrOut receives diagnostics, see [Writer.Err].
	ErrOut io.Writer
}
