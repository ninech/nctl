package update

import (
	"bytes"
	"strings"
	"testing"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestProject(t *testing.T) {
	t.Parallel()

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
				resourceCmd: resourceCmd{Name: projectName},
				DisplayName: ptr.To("some display name"),
			},
			checkProject: func(t *testing.T, cmd projectCmd, orig, updated *management.Project) {
				is := require.New(t)
				is.Equal(*cmd.DisplayName, updated.Spec.DisplayName)
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			out := &bytes.Buffer{}
			tc.cmd.Writer = format.NewWriter(out)

			apiClient, err := test.SetupClient(
				test.WithObjects(tc.orig),
				test.WithOrganization(organization),
				test.WithDefaultProject(tc.orig.Name),
				test.WithKubeconfig(t),
			)
			if err != nil {
				t.Fatal(err)
			}

			if err := tc.cmd.Run(t.Context(), apiClient); err != nil {
				t.Fatal(err)
			}

			updated := &management.Project{}
			if err := apiClient.Get(t.Context(), api.ObjectName(tc.orig), updated); err != nil {
				t.Fatal(err)
			}

			if tc.checkProject != nil {
				tc.checkProject(t, tc.cmd, tc.orig, updated)
			}

			if !strings.Contains(out.String(), "updated") {
				t.Errorf("expected output to contain 'updated', got %q", out.String())
			}
			if !strings.Contains(out.String(), projectName) {
				t.Errorf("expected output to contain project name %q, got %q", projectName, out.String())
			}
		})
	}
}
