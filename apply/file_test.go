package apply

import (
	"context"
	"fmt"
	"os"
	"testing"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	iam "github.com/ninech/apis/iam/v1alpha1"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

const (
	apiServiceAccountYAML = `kind: APIServiceAccount
apiVersion: iam.nine.ch/v1alpha1
metadata:
  name: %s
  namespace: default
  annotations:
    key: %s
spec:
  deletionPolicy: "%s"
`
	apiServiceAccountJSON = `{
    "apiVersion": "iam.nine.ch/v1alpha1",
    "kind": "APIServiceAccount",
    "metadata": {
        "name": "%s",
        "namespace": "default",
        "annotations": {
            "key": "%s"
        }
    },
    "spec": {
      "deletionPolicy": "%s"
    }
}
`
	missingKindResourceYAML = `
apiVersion: nope.nine.ch/v1alpha1
metadata:
  name: %s
  namespace: default
  annotations:
    key: %s
spec: {}
`

	invalidResourceJSON = `
{wat}
`
)

func TestFile(t *testing.T) {
	ctx := context.Background()
	apiClient, err := test.SetupClient()
	require.NoError(t, err)

	tests := map[string]struct {
		name              string
		file              string
		update            bool
		updatedAnnotation string
		updatedSpecValue  string
		expectedErr       bool
		delete            bool
	}{
		"create from yaml": {
			file: apiServiceAccountYAML,
		},
		"create from json": {
			file: apiServiceAccountJSON,
		},
		"apply from yaml": {
			file:              apiServiceAccountYAML,
			update:            true,
			updatedAnnotation: "updated",
			updatedSpecValue:  "Delete",
		},
		"apply from json": {
			file:              apiServiceAccountJSON,
			update:            true,
			updatedAnnotation: "updated",
			updatedSpecValue:  "Delete",
		},
		"create invalid yaml": {
			file:        missingKindResourceYAML,
			expectedErr: true,
		},
		"create invalid json": {
			file:        invalidResourceJSON,
			expectedErr: true,
		},
		"delete from yaml": {
			name:   "delete-yaml",
			file:   apiServiceAccountYAML,
			delete: true,
		},
		"delete from json": {
			name:   "delete-json",
			file:   apiServiceAccountJSON,
			delete: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			f, err := os.CreateTemp("", "nctl-filetest*")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(f.Name())

			if _, err := fmt.Fprintf(f, tc.file, name, "value", runtimev1.DeletionOrphan); err != nil {
				t.Fatal(err)
			}
			// The file is written, but the pointer is at the end.
			// Close it to flush content.
			require.NoError(t, f.Close())

			opts := []Option{}

			// For delete and update tests, we first create the resource.
			if tc.delete || tc.update {
				fileToCreate, err := os.Open(f.Name())
				require.NoError(t, err)
				err = File(ctx, apiClient, fileToCreate) // This will close fileToCreate
				require.NoError(t, err)
			}

			if tc.delete {
				opts = append(opts, Delete())
			}

			if tc.update {
				// Re-create the file to truncate it and write the updated content.
				updatedFile, err := os.Create(f.Name())
				require.NoError(t, err)
				_, err = fmt.Fprintf(updatedFile, tc.file, name, tc.updatedAnnotation, tc.updatedSpecValue)
				require.NoError(t, err)
				require.NoError(t, updatedFile.Close())

				opts = append(opts, UpdateOnExists())
			}

			fileToApply, err := os.Open(f.Name())
			require.NoError(t, err)

			if err := File(ctx, apiClient, fileToApply, opts...); err != nil {
				if tc.expectedErr {
					return
				}
				t.Fatal(err)
			} else if tc.expectedErr {
				t.Fatalf("expected error, got nil")
			}

			asa := &iam.APIServiceAccount{}
			if err := apiClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, asa); err != nil {
				if tc.delete {
					if !errors.IsNotFound(err) {
						t.Fatalf("expected resource to not exist after delete, got %s", err)
					}
					return
				}
				t.Fatalf("expected asa %s to exist, got: %s", tc.name, err)
			}

			if tc.update && asa.GetAnnotations()["key"] != tc.updatedAnnotation {
				t.Fatalf("expected annotation to be updated to %q, got %q",
					tc.updatedAnnotation, asa.GetAnnotations()["key"])
			}

			if tc.update && asa.GetDeletionPolicy() != runtimev1.DeletionDelete {
				t.Fatalf("expected spec.deletionPolicy to be updated to %q, got %q", "Delete", asa.GetDeletionPolicy())
			}
		})
	}
}
