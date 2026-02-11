package get

import (
	"bytes"
	"os"
	"strings"
	"testing"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api/config"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestProject(t *testing.T) {
	t.Parallel()
	organization := "evilcorp"

	for name, testCase := range map[string]struct {
		projects      []client.Object
		displayNames  []string
		name          string
		outputFormat  outputFormat
		allProjects   bool
		output        string
		errorContains []string
	}{
		"projects exist, full format": {
			projects:     test.Projects(organization, "dev", "staging", "prod"),
			displayNames: []string{"Development", "", "Production"},
			outputFormat: full,
			output: `PROJECT  DISPLAY NAME
dev      Development
prod     Production
staging  <none>
`,
		},
		"projects exist, no header format": {
			projects:     test.Projects(organization, "dev", "staging", "prod"),
			outputFormat: noHeader,
			output: `dev      <none>
prod     <none>
staging  <none>
`,
		},
		"projects exist and allProjects is set": {
			projects:     test.Projects(organization, "dev", "staging", "prod"),
			outputFormat: full,
			allProjects:  true,
			output: `PROJECT  DISPLAY NAME
dev      <none>
prod     <none>
staging  <none>
`,
		},
		"no projects exist": {
			projects:      []client.Object{},
			outputFormat:  full,
			errorContains: []string{`no "Projects" found`},
		},
		"no projects exist, no header format": {
			projects:      []client.Object{},
			outputFormat:  noHeader,
			errorContains: []string{`no "Projects" found`},
		},
		"specific project requested": {
			projects:     test.Projects(organization, "dev", "staging"),
			name:         "dev",
			outputFormat: full,
			output: `PROJECT  DISPLAY NAME
dev      <none>
`,
		},
		"specific project requested, but does not exist": {
			projects:      test.Projects(organization, "staging"),
			name:          "dev",
			outputFormat:  full,
			errorContains: []string{`no "Projects" found`},
		},
		"specific project requested, yaml output": {
			projects:     test.Projects(organization, "dev", "staging"),
			name:         "dev",
			outputFormat: yamlOut,
			output:       "metadata:\n  name: dev\n  namespace: evilcorp\nspec:\n  isNonProduction: false\nstatus:\n  atProvider: {}\n",
		},
		"specific project requested, json output": {
			projects:     test.Projects(organization, "dev", "staging"),
			name:         "dev",
			outputFormat: jsonOut,
			output: `{
  "metadata": {
    "name": "dev",
    "namespace": "evilcorp"
  },
  "spec": {
    "isNonProduction": false
  },
  "status": {
    "atProvider": {}
  }
}
`,
		},
		"no specific project requested, json output": {
			projects:     test.Projects(organization, "dev", "staging"),
			outputFormat: jsonOut,
			output: `[
  {
    "metadata": {
      "name": "dev",
      "namespace": "evilcorp"
    },
    "spec": {
      "isNonProduction": false
    },
    "status": {
      "atProvider": {}
    }
  },
  {
    "metadata": {
      "name": "staging",
      "namespace": "evilcorp"
    },
    "spec": {
      "isNonProduction": false
    },
    "status": {
      "atProvider": {}
    }
  }
]
`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			is := require.New(t)

			buf := &bytes.Buffer{}
			get := NewTestCmd(buf, testCase.outputFormat)
			get.AllProjects = testCase.allProjects

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
			is.NoError(err)

			cmd := projectCmd{
				resourceCmd: resourceCmd{
					Name: testCase.name,
				},
			}

			err = cmd.Run(t.Context(), apiClient, get)
			if len(testCase.errorContains) > 0 {
				is.Error(err)
				for _, s := range testCase.errorContains {
					is.Contains(strings.ToLower(err.Error()), strings.ToLower(s))
				}
				return
			}
			is.NoError(err)

			is.Equal(testCase.output, buf.String())
		})
	}
}

func TestProjectsConfigErrors(t *testing.T) {
	t.Parallel()

	is := require.New(t)
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
		output: output{Format: full},
	}
	// there is no kubeconfig so we expect to fail
	is.Error(cmd.Run(t.Context(), apiClient, get))

	// we create a kubeconfig which does not contain a nctl config
	// extension
	kubeconfig, err := test.CreateTestKubeconfig(apiClient, "")
	is.NoError(err)
	defer os.Remove(kubeconfig)
	is.ErrorIs(cmd.Run(t.Context(), apiClient, get), config.ErrExtensionNotFound)
}
