package create

import (
	"context"
	"os"
	"testing"
	"time"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/auth"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
)

func TestProjects(t *testing.T) {
	ctx := context.Background()
	projectName, organization := "testproject", "evilcorp"
	apiClient, err := test.SetupClient()
	if err != nil {
		t.Fatal(err)
	}
	kubeconfig, err := test.CreateTestKubeconfig(apiClient, organization)
	require.NoError(t, err)
	defer os.Remove(kubeconfig)

	cmd := projectCmd{
		Name:        projectName,
		DisplayName: "Some Display Name",
		Wait:        false,
		WaitTimeout: time.Second,
	}

	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}

	if err := apiClient.Get(
		ctx,
		api.NamespacedName(projectName, organization),
		&management.Project{},
	); err != nil {
		t.Fatalf("expected project %q to exist, got: %s", "testproject", err)
	}
}

func TestProjectsConfigErrors(t *testing.T) {
	ctx := context.Background()
	apiClient, err := test.SetupClient()
	if err != nil {
		t.Fatal(err)
	}
	cmd := projectCmd{
		Name:        "testproject",
		Wait:        false,
		WaitTimeout: time.Second,
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
