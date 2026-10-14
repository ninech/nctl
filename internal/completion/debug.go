package completion

import (
	"fmt"
	"io"
	"os"

	"github.com/ninech/nctl/api"
)

// DebugENV enables completion diagnostics on stderr.
//
// Predictors have to swallow every error, as anything they return ends up in
// the candidate list of the shell. Without diagnostics a misconfigured client
// is indistinguishable from a project holding no resources.
const DebugENV = "NCTL_COMPLETION_DEBUG"

// debugLog reports why a predictor predicts nothing.
// The zero value reports nothing.
type debugLog struct {
	w io.Writer
}

// newDebugLog returns a log writing to stderr if [DebugENV] is set.
func newDebugLog(stderr io.Writer) debugLog {
	if os.Getenv(DebugENV) == "" {
		return debugLog{}
	}

	return debugLog{w: stderr}
}

// fail reports err and predicts nothing,
// so that predictors can return it directly.
//
// The output is written on its own lines because the shell prints it in the
// middle of the line the user is currently completing.
func (l debugLog) fail(err error) []string {
	if err != nil && l.w != nil {
		fmt.Fprintf(l.w, "\n%s: completion: %v\n", api.Name, err)
	}

	return nil
}
