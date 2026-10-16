package completion

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDebugLogger(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	stderr := &bytes.Buffer{}
	log := NewDebugLogger(stderr)
	log.Debug("cannot list resources", "kind", "Postgres", "err", errors.New("no client"))
	log.Debug("second")

	is.Equal(
		"\nlevel=DEBUG msg=\"cannot list resources\" component=\"nctl completion\" kind=Postgres err=\"no client\"\n"+
			"\nlevel=DEBUG msg=second component=\"nctl completion\"\n",
		stderr.String(),
	)
}

// newTestLogger returns a debug logger writing to the output of t,
// so a failing test shows why a predictor predicted nothing,
// and to the returned buffer, so a test can assert why.
func newTestLogger(t testing.TB) (*slog.Logger, *bytes.Buffer) {
	t.Helper()

	buf := &bytes.Buffer{}
	return NewDebugLogger(io.MultiWriter(t.Output(), buf)), buf
}
