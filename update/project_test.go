package update

import (
	"context"
	"os"
	"testing"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		project      string
		cmd          projectCmd
		checkProject func(t *testing.T, cmd projectCmd, orig, updated *management.Project)
	}{
		"all fields update": {
			orig:    existingProject,
			project: projectName,
			cmd: projectCmd{
				resourceCmd: resourceCmd{Name: projectName},
				DisplayName: ptr.To("some display name"),
			},
			checkProject: func(t *testing.T, cmd projectCmd, orig, updated *management.Project) {
				assert.Equal(t, *cmd.DisplayName, updated.Spec.DisplayName)
			},
		},
	}

	for name, tc := range cases {
		tc := tc

		t.Run(name, func(t *testing.T) {
			apiClient, err := test.SetupClient(tc.orig)
			if err != nil {
				t.Fatal(err)
			}
			apiClient.Project = tc.project

			ctx := context.Background()

			// we create a kubeconfig which does not contain a nctl config
			// extension
			kubeconfig, err := test.CreateTestKubeconfig(apiClient, organization)
			require.NoError(t, err)
			defer os.Remove(kubeconfig)

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
