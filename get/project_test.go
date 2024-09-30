package get

import (
	"bytes"
	"context"
	"os"
	"testing"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/auth"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestProject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	organization := "evilcorp"

	for name, testCase := range map[string]struct {
		projects     []client.Object
		displayNames []string
		name         string
		outputFormat output
		allProjects  bool
		output       string
	}{
		"projects exist, full format": {
			projects:     test.Projects(organization, "dev", "staging", "prod"),
			displayNames: []string{"Development", "", "Production"},
			outputFormat: full,
			output: `NAME       DISPLAY NAME
dev        Development
prod       Production
staging    <none>
`,
		},
		"projects exist, no header format": {
			projects:     test.Projects(organization, "dev", "staging", "prod"),
			outputFormat: noHeader,
			output: `dev        <none>
prod       <none>
staging    <none>
`,
		},
		"projects exist and allProjects is set": {
			projects:     test.Projects(organization, "dev", "staging", "prod"),
			outputFormat: full,
			allProjects:  true,
			output: `NAME       DISPLAY NAME
dev        <none>
prod       <none>
staging    <none>
`,
		},
		"no projects exist": {
			projects:     []client.Object{},
			outputFormat: full,
			output:       "no Projects found\n",
		},
		"no projects exist, no header format": {
			projects:     []client.Object{},
			outputFormat: noHeader,
			output:       "no Projects found\n",
		},
		"specific project requested": {
			projects:     test.Projects(organization, "dev", "staging"),
			name:         "dev",
			outputFormat: full,
			output: `NAME    DISPLAY NAME
dev     <none>
`,
		},
		"specific project requested, but does not exist": {
			projects:     test.Projects(organization, "staging"),
			name:         "dev",
			outputFormat: full,
			output:       "no Projects found\n",
		},
		"specific project requested, yaml output": {
			projects:     test.Projects(organization, "dev", "staging"),
			name:         "dev",
			outputFormat: yamlOut,
			output:       "kind: Project\napiVersion: management.nine.ch/v1alpha1\nmetadata:\n  name: dev\n  namespace: evilcorp\nspec:\n  isNonProduction: false\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			testCase := testCase

			get := &Cmd{
				Output:      testCase.outputFormat,
				AllProjects: testCase.allProjects,
			}

			projects := testCase.projects
			for i, proj := range projects {
				if len(projects) != len(testCase.displayNames) {
					break
				}
				proj.(*management.Project).Spec.DisplayName = testCase.displayNames[i]
			}
			apiClient, err := test.SetupClient(
				test.WithObjects(projects...),
				test.WithKubeconfig(t),
				test.WithNameIndexFor(&management.Project{}),
				test.WithOrganization(organization),
			)
			require.NoError(t, err)

			buf := &bytes.Buffer{}
			cmd := projectCmd{
				resourceCmd: resourceCmd{
					Name: testCase.name,
				},
				out: buf,
			}

			if err := cmd.Run(ctx, apiClient, get); err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, testCase.output, buf.String())
		})
	}
}

func TestProjectsConfigErrors(t *testing.T) {
	ctx := context.Background()
	apiClient, err := test.SetupClient()
	if err != nil {
		t.Fatal(err)
	}
	cmd := projectCmd{
		resourceCmd: resourceCmd{
			Name: "testproject",
		},
	}
	get := &Cmd{
		Output: full,
	}
	// there is no kubeconfig so we expect to fail
	require.Error(t, cmd.Run(ctx, apiClient, get))

	// we create a kubeconfig which does not contain a nctl config
	// extension
	kubeconfig, err := test.CreateTestKubeconfig(apiClient, "")
	require.NoError(t, err)
	defer os.Remove(kubeconfig)
	require.ErrorIs(t, cmd.Run(ctx, apiClient, get), auth.ErrConfigNotFound)
}
