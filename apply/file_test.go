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
		tc := tc
		t.Run(name, func(t *testing.T) {
			f, err := os.CreateTemp("", "nctl-filetest*")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(f.Name())

			if _, err := f.WriteString(fmt.Sprintf(tc.file, name, "value", runtimev1.DeletionOrphan)); err != nil {
				t.Fatal(err)
			}

			opts := []Option{}

			if tc.delete {
				// we need to ensure that the resource exists before we can delete it
				if err := File(ctx, apiClient, f.Name()); err != nil {
					t.Fatal(err)
				}
				opts = append(opts, Delete())
			}

			if tc.update {
				// we need to ensure that the resource exists before we can update
				if err := File(ctx, apiClient, f.Name()); err != nil {
					t.Fatal(err)
				}

				if err := f.Truncate(0); err != nil {
					t.Fatal(err)
				}

				if _, err := f.Seek(0, 0); err != nil {
					t.Fatal(err)
				}

				if _, err := f.WriteString(fmt.Sprintf(tc.file, name, tc.updatedAnnotation, tc.updatedSpecValue)); err != nil {
					t.Fatal(err)
				}

				opts = append(opts, UpdateOnExists())
			}

			if err := File(ctx, apiClient, f.Name(), opts...); err != nil {
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
