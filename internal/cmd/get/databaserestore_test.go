package get

import (
	"bytes"
	"testing"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/internal/testutil"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDatabaseRestore(t *testing.T) {
	t.Parallel()

	dbA := meta.LocalTypedReference{
		LocalReference: meta.LocalReference{Name: "dba"},
		GroupKind:      metav1.GroupKind{Group: storage.Group, Kind: storage.PostgresDatabaseKind},
	}
	dbB := meta.LocalTypedReference{
		LocalReference: meta.LocalReference{Name: "dbb"},
		GroupKind:      metav1.GroupKind{Group: storage.Group, Kind: storage.MySQLDatabaseKind},
	}

	tests := []struct {
		name        string
		restores    []meta.LocalTypedReference
		target      string
		out         outputFormat
		wantContain []string
		wantLines   int
	}{
		{
			name:        "listAll",
			restores:    []meta.LocalTypedReference{dbA, dbB},
			wantContain: []string{"restore-a", "restore-b"},
			wantLines:   3,
		},
		{
			name:        "filterByTarget",
			restores:    []meta.LocalTypedReference{dbA, dbB},
			target:      "postgresdatabase/dba",
			wantContain: []string{"restore-a"},
			wantLines:   2,
		},
		{
			name:        "noHeader",
			restores:    []meta.LocalTypedReference{dbA, dbB},
			out:         noHeader,
			wantContain: []string{"restore-a", "restore-b"},
			wantLines:   2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			objects := []client.Object{}
			for i, ref := range tt.restores {
				objects = append(objects, testutil.DatabaseRestore("restore-"+string(rune('a'+i)), testutil.DefaultProject, "backup", ref))
			}

			apiClient := testutil.SetupClient(t, testutil.WithObjects(objects...))

			out := tt.out
			if out == "" {
				out = full
			}
			buf := &bytes.Buffer{}
			dbCmd := databaseRestoreCmd{}
			if tt.target != "" {
				dbCmd.Target = testutil.TypedReference(t, tt.target)
			}
			is.NoError(dbCmd.Run(t.Context(), apiClient, NewTestCmd(t, buf, out)))
			for _, substr := range tt.wantContain {
				is.Contains(buf.String(), substr)
			}
			is.Equal(tt.wantLines, testutil.CountLines(buf.String()))
		})
	}
}
