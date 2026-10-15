package completion

import (
	"io"
	"log/slog"

	"github.com/ninech/nctl/api"
)

// DebugENV enables completion diagnostics on stderr.
//
// Predictors have to swallow every error, as anything they return ends up in
// the candidate list of the shell. Without diagnostics a misconfigured client
// is indistinguishable from a project holding no resources.
const DebugENV = "NCTL_COMPLETION_DEBUG"

// NewDebugLogger returns a logger writing to w
// why predictors predict nothing.
//
// Every record starts on a new line, because the shell prints it in the
// middle of the line the user is currently completing.
func NewDebugLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(newlineWriter{w}, &slog.HandlerOptions{
		Level: slog.LevelDebug,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})).With("component", api.Name+" completion")
}

// newlineWriter prefixes every write with a newline.
// A [slog.TextHandler] writes each record with a single call to Write,
// so every record starts on a line of its own.
type newlineWriter struct {
	w io.Writer
}

func (n newlineWriter) Write(p []byte) (int, error) {
	if _, err := io.WriteString(n.w, "\n"); err != nil {
		return 0, err
	}

	return n.w.Write(p)
}
