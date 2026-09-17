package delete

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	iam "github.com/ninech/apis/iam/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/testutil"
	"github.com/stretchr/testify/require"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestFromFile(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	asa := &iam.APIServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "asa", Namespace: testutil.DefaultProject}}
	apiClient := testutil.SetupClient(t, testutil.WithObjects(asa))

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
	is.NoError(cmd.Run(t.Context(), apiClient))
	is.Contains(out.String(), "deleted APIServiceAccount asa/default")
	is.True(kerrors.IsNotFound(apiClient.Get(t.Context(), api.ObjectName(asa), asa)))
}
