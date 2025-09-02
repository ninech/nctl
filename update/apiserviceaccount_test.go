package update

import (
	"testing"

	iam "github.com/ninech/apis/iam/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestAPIServiceAccount(t *testing.T) {
	const (
		asaName      = "some-asa"
		organization = "org"
	)

	for name, tc := range map[string]struct {
		orig                   *iam.APIServiceAccount
		cmd                    apiServiceAccountCmd
		checkAPIServiceAccount func(t *testing.T, cmd apiServiceAccountCmd, orig, updated *iam.APIServiceAccount)
	}{
		"all fields update": {
			orig: &iam.APIServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      asaName,
					Namespace: organization,
				},
				Spec: iam.APIServiceAccountSpec{},
			},
			cmd: apiServiceAccountCmd{
				resourceCmd:        resourceCmd{Name: asaName},
				OrganizationAccess: ptr.To(true),
			},
			checkAPIServiceAccount: func(t *testing.T, cmd apiServiceAccountCmd, orig, updated *iam.APIServiceAccount) {
				assert.Equal(t, *cmd.OrganizationAccess, updated.Spec.ForProvider.OrganizationAccess)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			apiClient, err := test.SetupClient(
				test.WithObjects(tc.orig),
				test.WithOrganization(organization),
				test.WithDefaultProject(organization),
				test.WithKubeconfig(t),
			)
			if err != nil {
				t.Fatal(err)
			}

			if err := tc.cmd.Run(t.Context(), apiClient); err != nil {
				t.Fatal(err)
			}

			updated := &iam.APIServiceAccount{}
			if err := apiClient.Get(t.Context(), api.ObjectName(tc.orig), updated); err != nil {
				t.Fatal(err)
			}

			if tc.checkAPIServiceAccount != nil {
				tc.checkAPIServiceAccount(t, tc.cmd, tc.orig, updated)
			}
		})
	}
}
