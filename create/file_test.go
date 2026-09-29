package create

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
)

func TestFromFile(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	path := filepath.Join(t.TempDir(), "manifest.yaml")
	is.NoError(os.WriteFile(path, []byte(`kind: APIServiceAccount
apiVersion: iam.nine.ch/v1alpha1
metadata:
  name: asa
  namespace: default
`), 0o600))
	f, err := os.Open(path)
	is.NoError(err)

	out := &bytes.Buffer{}
	cmd := &fromFile{Writer: format.NewWriter(out), Filename: f}
	is.NoError(cmd.Run(t.Context(), test.SetupClient(t)))
	is.Contains(out.String(), "created APIServiceAccount asa/default")
	is.ErrorIs(f.Close(), os.ErrClosed, "file is closed")
}
