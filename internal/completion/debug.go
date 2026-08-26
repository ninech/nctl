package completion

import (
	"fmt"
	"os"

	"github.com/ninech/nctl/internal/cli"
)

// DebugENV enables completion diagnostics on stderr.
//
// Predictors have to swallow every error, as anything they return ends up in
// the candidate list of the shell. Without diagnostics a misconfigured client
// is indistinguishable from a project holding no resources.
const DebugENV = "NCTL_COMPLETION_DEBUG"

// fail reports err on stderr if [DebugENV] is set and predicts nothing, so
// that predictors can return it directly.
//
// The output is written on its own lines because the shell prints it in the
// middle of the line the user is currently completing.
func fail(err error) []string {
	if err != nil && os.Getenv(DebugENV) != "" {
		fmt.Fprintf(os.Stderr, "\n%s: completion: %v\n", cli.Name, err)
	}

	return nil
}
