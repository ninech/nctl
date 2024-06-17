// as we need the test.CreateTestKubeconfig function and to not cause a cycle,
// we use the auth_test package
package auth_test

import (
	"context"
	"os"
	"testing"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/auth"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestWhoAmICmd_Run(t *testing.T) {
	client := fake.NewClientBuilder().Build()
	apiClient := &api.Client{WithWatch: client, Project: "default", KubeconfigPath: "*-kubeconfig.yaml"}
	apiClient.Config = &rest.Config{BearerToken: auth.FakeJWTToken}

	kubeconfig, err := test.CreateTestKubeconfig(apiClient, "test")
	require.NoError(t, err)
	defer os.Remove(kubeconfig)

	s := &auth.WhoAmICmd{
		IssuerURL: "https://auth.nine.ch/auth/realms/pub",
		ClientID:  "nineapis.ch-f178254",
	}

	err = s.Run(context.Background(), apiClient)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	require.NoError(t, err)
}
