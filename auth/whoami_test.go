// as we need the test.CreateTestKubeconfig function and to not cause a cycle,
// we use the auth_test package
package auth_test

import (
	"context"
	"testing"

	"github.com/ninech/nctl/auth"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
)

func TestWhoAmICmd_Run(t *testing.T) {
	apiClient, err := test.SetupClient(
		test.WithKubeconfig(t),
	)
	require.NoError(t, err)

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
