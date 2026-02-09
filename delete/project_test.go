package delete

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/config"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

func TestProject(t *testing.T) {
	t.Parallel()
	organization := "evilcorp"
	for name, testCase := range map[string]struct {
		projects      []string
		name          string
		errorExpected bool
		errorCheck    func(err error) bool
	}{
		"happy path": {
			projects:      []string{"dev", "staging"},
			name:          "dev",
			errorExpected: false,
		},
		"project does not exist": {
			projects:      []string{"staging"},
			name:          "dev",
			errorExpected: true,
			errorCheck: func(err error) bool {
				return kerrors.IsNotFound(err)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			out := &bytes.Buffer{}
			cmd := projectCmd{
				resourceCmd: resourceCmd{
					Writer: format.NewWriter(out),
					Force:  true,
					Wait:   false,
					Name:   testCase.name,
				},
			}

			apiClient, err := test.SetupClient(
				test.WithOrganization(organization),
				test.WithProjects(testCase.projects...),
				test.WithKubeconfig(t),
			)
			if err != nil {
				t.Fatalf("failed to setup api client: %v", err)
			}

			ctx := t.Context()
			err = cmd.Run(ctx, apiClient)
			if testCase.errorExpected {
				if err == nil {
					t.Fatal("expected error but got none")
				}
				if !errorCheck(testCase.errorCheck)(err) {
					t.Fatalf("error check failed for error: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error while running project delete: %v", err)
			}

			if !kerrors.IsNotFound(
				apiClient.Get(
					ctx,
					api.NamespacedName(testCase.name, organization),
					&management.Project{},
				),
			) {
				t.Fatal("expected project to be deleted, but it still exists")
			}

			if !strings.Contains(out.String(), "deletion started") {
				t.Errorf("expected output to contain 'deletion started', got %q", out.String())
			}
			if !strings.Contains(out.String(), testCase.name) {
				t.Errorf("expected output to contain project name %q, got %q", testCase.name, out.String())
			}
		})
	}
}

func TestProjectsConfigErrors(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	apiClient, err := test.SetupClient()
	if err != nil {
		t.Fatalf("failed to setup api client: %v", err)
	}
	cmd := projectCmd{
		resourceCmd: resourceCmd{
			Force: true,
			Wait:  false,
			Name:  "test",
		},
	}
	// there is no kubeconfig so we expect to fail
	if err := cmd.Run(ctx, apiClient); err == nil {
		t.Error("expected error but got none")
	}

	// we create a kubeconfig which does not contain a nctl config
	// extension
	kubeconfig, err := test.CreateTestKubeconfig(apiClient, "")
	if err != nil {
		t.Fatalf("failed to create test kubeconfig: %v", err)
	}
	defer os.Remove(kubeconfig)
	if err := cmd.Run(ctx, apiClient); !errors.Is(err, config.ErrExtensionNotFound) {
		t.Errorf("expected error %v, got %v", config.ErrExtensionNotFound, err)
	}
}

// errorCheck defaults the given errCheck function if it is nil. The returned
// function will return true for every passed error.
func errorCheck(errCheck func(err error) bool) func(err error) bool {
	if errCheck == nil {
		return func(_ error) bool { return true }
	}
	return errCheck
}
