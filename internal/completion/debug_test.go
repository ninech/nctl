package completion

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDebugLog(t *testing.T) {
	err := errors.New("no client")

	t.Run("reports to stderr when enabled", func(t *testing.T) {
		t.Setenv(DebugENV, "1")
		is := require.New(t)

		stderr := &bytes.Buffer{}
		is.Nil(newDebugLog(stderr).fail(err))
		is.Equal("\nnctl: completion: no client\n", stderr.String())
	})

	t.Run("reports nothing when disabled", func(t *testing.T) {
		t.Setenv(DebugENV, "")
		is := require.New(t)

		stderr := &bytes.Buffer{}
		is.Nil(newDebugLog(stderr).fail(err))
		is.Empty(stderr.String())
	})

	t.Run("zero value reports nothing", func(t *testing.T) {
		require.Nil(t, debugLog{}.fail(err))
	})
}
