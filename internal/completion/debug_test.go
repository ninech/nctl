package completion

import (
	"bytes"
	"errors"
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
