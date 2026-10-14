package testutil

import (
	"io"

	"github.com/alecthomas/kong"
)

// KongContext returns a Kong context writing to stdout and stderr,
// for calling the BeforeApply hooks of commands directly.
func KongContext(stdout, stderr io.Writer) *kong.Context {
	return &kong.Context{Kong: &kong.Kong{Stdout: stdout, Stderr: stderr}}
}
