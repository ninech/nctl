// as we need the test.CreateTestKubeconfig function and to not cause a cycle,
// we use the auth_test package
package auth_test

import (
	"testing"

	"github.com/ninech/nctl/auth"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
)

func TestWhoAmICmd_Run(t *testing.T) {
	t.Parallel()

	is := require.New(t)
	apiClient, err := test.SetupClient(
		test.WithKubeconfig(t),
	)
	is.NoError(err)

	s := &auth.WhoAmICmd{
		IssuerURL: "https://auth.nine.ch/auth/realms/pub",
		ClientID:  "nineapis.ch-f178254",
	}

	is.NoError(s.Run(t.Context(), apiClient))
}
