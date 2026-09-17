package api_test

import (
	"fmt"
	"os"
	"testing"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	iam "github.com/ninech/apis/iam/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	labeledAPIServiceAccountYAML = `kind: APIServiceAccount
apiVersion: iam.nine.ch/v1alpha1
metadata:
  name: %s
  namespace: default
  annotations:
    key: value
  labels:
    team: platform
`
)

func TestFromFile(t *testing.T) {
	t.Parallel()

	apiClient := test.SetupClient(t)

	tests := map[string]struct {
		file           string
		create         bool
		apply          bool
		delete         bool
		expectedResult api.ApplyResult
		// expectedErr expects the create to fail; no other step runs then.
		expectedErr bool
	}{
		"create from yaml": {
			file:   apiServiceAccountYAML,
			create: true,
		},
		"create from json": {
			file:   apiServiceAccountJSON,
			create: true,
		},
		"create invalid yaml": {
			file:        missingKindResourceYAML,
			create:      true,
			expectedErr: true,
		},
		"create invalid json": {
			file:        invalidResourceJSON,
			create:      true,
			expectedErr: true,
		},
		"apply creates from yaml": {
			file:           apiServiceAccountYAML,
			apply:          true,
			expectedResult: api.ApplyResultCreated,
		},
		"apply updates from yaml": {
			file:           apiServiceAccountYAML,
			create:         true,
			apply:          true,
			expectedResult: api.ApplyResultUpdated,
		},
		"apply updates from json": {
			file:           apiServiceAccountJSON,
			create:         true,
			apply:          true,
			expectedResult: api.ApplyResultUpdated,
		},
		"delete from yaml": {
			file:   apiServiceAccountYAML,
			create: true,
			delete: true,
		},
		"delete from json": {
			file:   apiServiceAccountJSON,
			create: true,
			delete: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)
			ctx := t.Context()

			if tc.create {
				f := manifest(t, tc.file, name, "value", runtimev1.DeletionOrphan)
				obj, err := apiClient.CreateFromFile(ctx, f)
				is.ErrorIs(f.Close(), os.ErrClosed, "file is closed")
				if tc.expectedErr {
					is.Error(err)
					return
				}
				is.NoError(err)
				assertObj(t, obj, name)
			}

			if tc.apply {
				// The updated manifest changes the annotation and the spec.
				f := manifest(t, tc.file, name, "updated", runtimev1.DeletionDelete)
				obj, result, err := apiClient.ApplyFromFile(ctx, f)
				is.ErrorIs(f.Close(), os.ErrClosed, "file is closed")
				is.NoError(err)
				is.Equal(tc.expectedResult, result)
				assertObj(t, obj, name)
			}

			if tc.delete {
				f := manifest(t, tc.file, name, "value", runtimev1.DeletionOrphan)
				obj, err := apiClient.DeleteFromFile(ctx, f)
				is.ErrorIs(f.Close(), os.ErrClosed, "file is closed")
				is.NoError(err)
				assertObj(t, obj, name)
			}

			asa := &iam.APIServiceAccount{}
			err := apiClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, asa)
			if tc.delete {
				is.True(errors.IsNotFound(err), "expected resource to not exist after delete, got %v", err)
				return
			}
			is.NoError(err)

			if tc.apply {
				is.Equal("updated", asa.GetAnnotations()["key"])
				is.Equal(runtimev1.DeletionDelete, asa.GetDeletionPolicy())
			} else {
				is.Equal("value", asa.GetAnnotations()["key"])
				is.Equal(runtimev1.DeletionOrphan, asa.GetDeletionPolicy())
			}
		})
	}
}

// TestApplyOverObjectWithoutMetadata applies a manifest which sets annotations
// and labels over an existing object which has neither.
func TestApplyOverObjectWithoutMetadata(t *testing.T) {
	t.Parallel()
	is := require.New(t)
	ctx := t.Context()

	const name = "bare"
	existing := &iam.APIServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
	apiClient := test.SetupClient(t, test.WithObjects(existing))

	f := manifest(t, labeledAPIServiceAccountYAML, name)
	obj, result, err := apiClient.ApplyFromFile(ctx, f)
	is.NoError(err)
	is.Equal(api.ApplyResultUpdated, result)
	assertObj(t, obj, name)

	asa := &iam.APIServiceAccount{}
	is.NoError(apiClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, asa))
	is.Equal("value", asa.GetAnnotations()["key"])
	is.Equal(map[string]string{"team": "platform"}, asa.GetLabels())
}

func TestFromFileMissing(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	apiClient := test.SetupClient(t)
	ctx := t.Context()

	_, err := apiClient.CreateFromFile(ctx, nil)
	is.EqualError(err, "no manifest file given")
	_, _, err = apiClient.ApplyFromFile(ctx, nil)
	is.EqualError(err, "no manifest file given")
	_, err = apiClient.DeleteFromFile(ctx, nil)
	is.EqualError(err, "no manifest file given")
}

// manifest writes the template filled with args to a temporary file and
// returns it opened for reading.
func manifest(t *testing.T, template string, args ...any) *os.File {
	t.Helper()

	path := t.TempDir() + "/manifest"
	require.NoError(t, os.WriteFile(path, fmt.Appendf(nil, template, args...), 0o600))
	f, err := os.Open(path)
	require.NoError(t, err)

	return f
}

func assertObj(t *testing.T, obj *unstructured.Unstructured, name string) {
	t.Helper()

	require.NotNil(t, obj)
	require.Equal(t, "APIServiceAccount", obj.GetKind())
	require.Equal(t, name, obj.GetName())
	require.Equal(t, "default", obj.GetNamespace())
}
