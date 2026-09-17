package apply

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/testutil"
	"github.com/stretchr/testify/require"
)

const apiServiceAccountYAML = `kind: APIServiceAccount
apiVersion: iam.nine.ch/v1alpha1
metadata:
  name: asa
  namespace: default
`

func TestFromFile(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	apiClient := testutil.SetupClient(t)
	ctx := t.Context()

	out := &bytes.Buffer{}
	cmd := &fromFile{Writer: format.NewWriter(out), Filename: manifest(t, apiServiceAccountYAML)}
	is.NoError(cmd.Run(ctx, apiClient))
	is.Contains(out.String(), "created APIServiceAccount asa/default")

	out.Reset()
	cmd.Filename = manifest(t, apiServiceAccountYAML)
	is.NoError(cmd.Run(ctx, apiClient))
	is.Contains(out.String(), "applied APIServiceAccount asa/default")
}

func manifest(t *testing.T, content string) *os.File {
	t.Helper()

	path := filepath.Join(t.TempDir(), "manifest.yaml")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	f, err := os.Open(path)
	require.NoError(t, err)

	return f
}
