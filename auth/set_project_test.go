package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestOrgFromProject(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		objects []client.Object
		project string
		wantOrg string
		wantErr bool
	}{
		"simple project name matches org": {
			objects: nil,
			project: "test",
			wantOrg: "test",
		},
		"simple project name not in user orgs": {
			objects: nil,
			project: "unknown",
			wantErr: true,
		},
		"project with dash found in org": {
			objects: []client.Object{
				testProject("test-prod", "test"),
			},
			project: "test-prod",
			wantOrg: "test",
		},
		"project with dash found in second org": {
			objects: []client.Object{
				testProject("bla-staging", "bla"),
			},
			project: "bla-staging",
			wantOrg: "bla",
		},
		"project with dash not found in any org": {
			objects: nil,
			project: "test-nonexistent",
			wantErr: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			apiClient, err := test.SetupClient(
				test.WithObjects(tc.objects...),
			)
			is.NoError(err)

			org, err := orgFromProject(t.Context(), apiClient, tc.project)
			if tc.wantErr {
				is.Error(err)
				return
			}
			is.NoError(err)
			is.Equal(tc.wantOrg, org)
		})
	}
}

func TestOrgFromProjectAPIErrors(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		interceptor    interceptor.Funcs
		wantErrContain string
	}{
		"NotFound results in not found error": {
			interceptor: interceptor.Funcs{
				Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
				},
			},
			wantErrContain: "could not find project",
		},
		"Forbidden results in not found error": {
			interceptor: interceptor.Funcs{
				Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return apierrors.NewForbidden(schema.GroupResource{}, key.Name, errors.New("forbidden"))
				},
			},
			wantErrContain: "could not find project",
		},
		"propagates other errors": {
			interceptor: interceptor.Funcs{
				Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return errors.New("connection refused")
				},
			},
			wantErrContain: "connection refused",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			apiClient, err := test.SetupClient(
				test.WithInterceptorFuncs(tc.interceptor),
			)
			is.NoError(err)

			_, err = orgFromProject(t.Context(), apiClient, "test-prod")
			is.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.wantErrContain))
		})
	}
}

func TestTrySwitchOrg(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		objects    []client.Object
		currentOrg string
		project    string
		wantErr    bool
	}{
		"switches to org containing project": {
			objects:    []client.Object{testProject("bla-prod", "bla")},
			currentOrg: "test",
			project:    "bla-prod",
		},
		"project already in current org": {
			objects:    []client.Object{testProject("test-dev", "test")},
			currentOrg: "test",
			project:    "test-dev",
		},
		"simple project name switches to matching org": {
			objects:    nil,
			currentOrg: "bla",
			project:    "test",
		},
		"simple project name not in user orgs": {
			objects:    nil,
			currentOrg: "test",
			project:    "unknown",
			wantErr:    true,
		},
		"project with prefix matching current org": {
			objects:    []client.Object{testProject("test-staging", "test")},
			currentOrg: "bla",
			project:    "test-staging",
		},
		"error when project not found": {
			objects:    nil,
			currentOrg: "test",
			project:    "nonexistent-project",
			wantErr:    true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			apiClient, err := test.SetupClient(
				test.WithOrganization(tc.currentOrg),
				test.WithKubeconfig(t),
				test.WithObjects(tc.objects...),
			)
			is.NoError(err)

			err = trySwitchOrg(t.Context(), apiClient, tc.project)
			if tc.wantErr {
				is.Error(err)
				return
			}
			is.NoError(err)
		})
	}
}

func TestSetProjectCmd(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		objects    []client.Object
		currentOrg string
		project    string
		wantErr    bool
	}{
		"project exists in current org": {
			objects:    []client.Object{testProject("test-dev", "test")},
			currentOrg: "test",
			project:    "test-dev",
		},
		"project exists in different org": {
			objects:    []client.Object{testProject("bla-prod", "bla")},
			currentOrg: "test",
			project:    "bla-prod",
		},
		"project does not exist anywhere": {
			objects:    nil,
			currentOrg: "test",
			project:    "nonexistent",
			wantErr:    true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			apiClient, err := test.SetupClient(
				test.WithOrganization(tc.currentOrg),
				test.WithKubeconfig(t),
				test.WithObjects(tc.objects...),
			)
			is.NoError(err)

			cmd := SetProjectCmd{Name: tc.project}
			err = cmd.Run(t.Context(), apiClient)
			if tc.wantErr {
				is.Error(err)
				return
			}
			is.NoError(err)
		})
	}
}

func testProject(name, namespace string) *management.Project {
	return &management.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}
