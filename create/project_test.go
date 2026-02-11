package create

import (
	"os"
	"testing"
	"time"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/config"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
)

func TestProjects(t *testing.T) {
	t.Parallel()

	const existsAlready = "exists-already"
	projectName, organization := "testproject", "evilcorp"
	apiClient := test.SetupClient(t,
		test.WithOrganization("evilcorp"),
		test.WithKubeconfig(),
		test.WithProjects(existsAlready),
	)

	cmd := projectCmd{
		resourceCmd: resourceCmd{
			Name:        projectName,
			Wait:        false,
			WaitTimeout: time.Second,
		},
		DisplayName: "Some Display Name",
	}

	if err := cmd.Run(t.Context(), apiClient); err != nil {
		t.Fatal(err)
	}

	if err := apiClient.Get(
		t.Context(),
		api.NamespacedName(projectName, organization),
		&management.Project{},
	); err != nil {
		t.Fatalf("expected project %q to exist, got: %s", "testproject", err)
	}

	// test if the command errors out if the project already exists
	cmd.Name = existsAlready
	if err := cmd.Run(t.Context(), apiClient); err == nil {
		t.Fatal("expected an error as project already exists, but got none")
	}
}

func TestProjectsConfigErrors(t *testing.T) {
	t.Parallel()

	is := require.New(t)
	apiClient := test.SetupClient(t)
	cmd := projectCmd{
		resourceCmd: resourceCmd{
			Name:        "testproject",
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}
	// there is no kubeconfig so we expect to fail
	is.Error(cmd.Run(t.Context(), apiClient))

	// we create a kubeconfig which does not contain a nctl config
	// extension
	kubeconfig, err := test.CreateTestKubeconfig(apiClient, "")
	is.NoError(err)
	defer os.Remove(kubeconfig)
	is.ErrorIs(cmd.Run(t.Context(), apiClient), config.ErrExtensionNotFound)
}
