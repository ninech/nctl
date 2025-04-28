package update

import (
	"context"
	"testing"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/common"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestProject(t *testing.T) {
	const (
		projectName  = "some-project"
		organization = "org"
	)
	existingProject := &management.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:      projectName,
			Namespace: organization,
		},
		Spec: management.ProjectSpec{},
	}

	cases := map[string]struct {
		orig         *management.Project
		cmd          projectCmd
		checkProject func(t *testing.T, cmd projectCmd, orig, updated *management.Project)
	}{
		"all fields update": {
			orig: existingProject,
			cmd: projectCmd{
				ProjectCmd: common.ProjectCmd{
					DisplayName: ptr.To("some Display Name"),
				},
				resourceCmd: resourceCmd{
					Name: projectName,
				},
			},
			checkProject: func(t *testing.T, cmd projectCmd, orig, updated *management.Project) {
				assert.Equal(t, *cmd.DisplayName, updated.Spec.DisplayName)
			},
		},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			apiClient, err := test.SetupClient(
				test.WithObjects(tc.orig),
				test.WithOrganization(organization),
				test.WithDefaultProject(tc.orig.Name),
				test.WithKubeconfig(t),
			)
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			if err := tc.cmd.Run(ctx, apiClient); err != nil {
				t.Fatal(err)
			}

			updated := &management.Project{}
			if err := apiClient.Get(ctx, api.ObjectName(tc.orig), updated); err != nil {
				t.Fatal(err)
			}

			if tc.checkProject != nil {
				tc.checkProject(t, tc.cmd, tc.orig, updated)
			}
		})
	}
}
