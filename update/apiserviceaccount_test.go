package update

import (
	"bytes"
	"strings"
	"testing"

	iam "github.com/ninech/apis/iam/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestAPIServiceAccount(t *testing.T) {
	t.Parallel()

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
				is := require.New(t)
				is.Equal(*cmd.OrganizationAccess, updated.Spec.ForProvider.OrganizationAccess)
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			out := &bytes.Buffer{}
			tc.cmd.Writer = format.NewWriter(out)

			apiClient := test.SetupClient(t,
				test.WithObjects(tc.orig),
				test.WithOrganization(organization),
				test.WithDefaultProject(organization),
				test.WithKubeconfig(),
			)

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

			if !strings.Contains(out.String(), "updated") {
				t.Errorf("expected output to contain 'updated', got %q", out.String())
			}
			if !strings.Contains(out.String(), asaName) {
				t.Errorf("expected output to contain %q, got %q", asaName, out.String())
			}
		})
	}
}
