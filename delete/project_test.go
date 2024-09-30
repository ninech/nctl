package delete

import (
	"context"
	"os"
	"testing"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/auth"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
)

func TestProject(t *testing.T) {
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
				return errors.IsNotFound(err)
			},
		},
	} {
		testCase := testCase
		t.Run(name, func(t *testing.T) {
			cmd := projectCmd{
				resourceCmd: resourceCmd{
					Force: true,
					Wait:  false,
					Name:  testCase.name,
				},
			}

			apiClient, err := test.SetupClient(
				test.WithOrganization(organization),
				test.WithProjects(testCase.projects...),
				test.WithKubeconfig(t),
			)
			require.NoError(t, err)

			ctx := context.Background()
			err = cmd.Run(ctx, apiClient)
			if testCase.errorExpected {
				require.Error(t, err)
				require.True(t, errorCheck(testCase.errorCheck)(err))
				return
			} else {
				require.NoError(t, err)
			}

			require.True(t, errors.IsNotFound(
				apiClient.Get(
					ctx,
					api.NamespacedName(testCase.name, organization),
					&management.Project{},
				),
			))
		})
	}
}

func TestProjectsConfigErrors(t *testing.T) {
	ctx := context.Background()
	apiClient, err := test.SetupClient()
	if err != nil {
		t.Fatal(err)
	}
	cmd := projectCmd{
		resourceCmd: resourceCmd{
			Force: true,
			Wait:  false,
			Name:  "test",
		},
	}
	// there is no kubeconfig so we expect to fail
	require.Error(t, cmd.Run(ctx, apiClient))

	// we create a kubeconfig which does not contain a nctl config
	// extension
	kubeconfig, err := test.CreateTestKubeconfig(apiClient, "")
	require.NoError(t, err)
	defer os.Remove(kubeconfig)
	require.ErrorIs(t, cmd.Run(ctx, apiClient), auth.ErrConfigNotFound)
}

// errorCheck defaults the given errCheck function if it is nil. The returned
// function will return true for every passed error.
func errorCheck(errCheck func(err error) bool) func(err error) bool {
	if errCheck == nil {
		return func(_ error) bool { return true }
	}
	return errCheck
}
