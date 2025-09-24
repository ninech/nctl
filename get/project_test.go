package get

import (
	"bytes"
	"context"
	"os"
	"regexp"
	"testing"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api/config"
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
		outputFormat outputFormat
		allProjects  bool
		output       string
		expectRegexp *regexp.Regexp
	}{
		"projects exist, full format": {
			projects:     test.Projects(organization, "dev", "staging", "prod"),
			displayNames: []string{"Development", "", "Production"},
			outputFormat: full,
			expectRegexp: regexp.MustCompile(`PROJECT\s+DISPLAY NAME\ndev\s+Development\nprod\s+Production\nstaging\s+<none>\n`),
		},
		"projects exist, no header format": {
			projects:     test.Projects(organization, "dev", "staging", "prod"),
			outputFormat: noHeader,
			expectRegexp: regexp.MustCompile(`dev\s+<none>\nprod\s+<none>\nstaging\s+<none>\n`),
		},
		"projects exist and allProjects is set": {
			projects:     test.Projects(organization, "dev", "staging", "prod"),
			outputFormat: full,
			allProjects:  true,
			expectRegexp: regexp.MustCompile(`PROJECT\s+DISPLAY NAME\ndev\s+<none>\nprod\s+<none>\nstaging\s+<none>\n`),
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
			expectRegexp: regexp.MustCompile(`PROJECT\s+DISPLAY NAME\ndev\s+<none>\n`),
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
			output:       "apiVersion: management.nine.ch/v1alpha1\nkind: Project\nmetadata:\n  creationTimestamp: null\n  name: dev\n  namespace: evilcorp\nspec:\n  isNonProduction: false\nstatus:\n  atProvider: {}\n",
		},
		"specific project requested, json output": {
			projects:     test.Projects(organization, "dev", "staging"),
			name:         "dev",
			outputFormat: jsonOut,
			output: `{
  "kind": "Project",
  "apiVersion": "management.nine.ch/v1alpha1",
  "metadata": {
    "name": "dev",
    "namespace": "evilcorp",
    "creationTimestamp": null
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
    "kind": "Project",
    "apiVersion": "management.nine.ch/v1alpha1",
    "metadata": {
      "name": "dev",
      "namespace": "evilcorp",
      "creationTimestamp": null
    },
    "spec": {
      "isNonProduction": false
    },
    "status": {
      "atProvider": {}
    }
  },
  {
    "kind": "Project",
    "apiVersion": "management.nine.ch/v1alpha1",
    "metadata": {
      "name": "staging",
      "namespace": "evilcorp",
      "creationTimestamp": null
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
			testCase := testCase

			buf := &bytes.Buffer{}
			get := &Cmd{
				output: output{
					Format:      testCase.outputFormat,
					AllProjects: testCase.allProjects,
					writer:      buf,
				},
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

			cmd := projectCmd{
				resourceCmd: resourceCmd{
					Name: testCase.name,
				},
			}

			if err := cmd.Run(ctx, apiClient, get); err != nil {
				t.Fatal(err)
			}

			if testCase.expectRegexp != nil {
				assert.Regexp(t, testCase.expectRegexp, buf.String())
			} else {
				assert.Equal(t, testCase.output, buf.String())
			}
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
		output: output{Format: full},
	}
	// there is no kubeconfig so we expect to fail
	require.Error(t, cmd.Run(ctx, apiClient, get))

	// we create a kubeconfig which does not contain a nctl config
	// extension
	kubeconfig, err := test.CreateTestKubeconfig(apiClient, "")
	require.NoError(t, err)
	defer os.Remove(kubeconfig)
	require.ErrorIs(t, cmd.Run(ctx, apiClient, get), config.ErrExtensionNotFound)
}
