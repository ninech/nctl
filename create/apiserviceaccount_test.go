package create

import (
	"testing"

	iam "github.com/ninech/apis/iam/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIServiceAccount(t *testing.T) {
	apiClient, err := test.SetupClient()
	require.NoError(t, err)

	for name, tc := range map[string]struct {
		cmd                    apiServiceAccountCmd
		checkAPIServiceAccount func(t *testing.T, cmd apiServiceAccountCmd, asa *iam.APIServiceAccount)
	}{
		"no org access": {
			cmd: apiServiceAccountCmd{
				resourceCmd: resourceCmd{Name: "no-org-access"},
			},
			checkAPIServiceAccount: func(t *testing.T, cmd apiServiceAccountCmd, asa *iam.APIServiceAccount) {
				assert.Equal(t, false, asa.Spec.ForProvider.OrganizationAccess)
			},
		},
		"org access": {
			cmd: apiServiceAccountCmd{
				resourceCmd:        resourceCmd{Name: "org-access"},
				OrganizationAccess: true,
			},
			checkAPIServiceAccount: func(t *testing.T, cmd apiServiceAccountCmd, asa *iam.APIServiceAccount) {
				assert.Equal(t, true, asa.Spec.ForProvider.OrganizationAccess)
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
