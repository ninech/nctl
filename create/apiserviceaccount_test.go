package create

import (
	"testing"

	iam "github.com/ninech/apis/iam/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
)

func TestAPIServiceAccount(t *testing.T) {
	t.Parallel()

	is := require.New(t)

	apiClient, err := test.SetupClient()
	is.NoError(err)

	for name, tc := range map[string]struct {
		cmd                    apiServiceAccountCmd
		checkAPIServiceAccount func(t *testing.T, cmd apiServiceAccountCmd, asa *iam.APIServiceAccount)
	}{
		"no org access": {
			cmd: apiServiceAccountCmd{
				resourceCmd: resourceCmd{Name: "no-org-access"},
			},
			checkAPIServiceAccount: func(t *testing.T, cmd apiServiceAccountCmd, asa *iam.APIServiceAccount) {
				is := require.New(t)
				is.Equal(false, asa.Spec.ForProvider.OrganizationAccess)
			},
		},
		"org access": {
			cmd: apiServiceAccountCmd{
				resourceCmd:        resourceCmd{Name: "org-access"},
				OrganizationAccess: true,
			},
			checkAPIServiceAccount: func(t *testing.T, cmd apiServiceAccountCmd, asa *iam.APIServiceAccount) {
				is := require.New(t)
				is.Equal(true, asa.Spec.ForProvider.OrganizationAccess)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.cmd.Run(t.Context(), apiClient); err != nil {
				t.Fatal(err)
			}
			created := &iam.APIServiceAccount{}
			if err := apiClient.Get(t.Context(), api.NamespacedName(tc.cmd.Name, apiClient.Project), created); err != nil {
				t.Fatal(err)
			}
			if tc.checkAPIServiceAccount != nil {
				tc.checkAPIServiceAccount(t, tc.cmd, created)
			}
		})
	}
}
