package get

import (
	"bytes"
	"context"
	"os"
	"testing"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestProject(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	organization := "evilcorp"

	for name, testCase := range map[string]struct {
		projects      []client.Object
		name          string
		outputFormat  output
		allNamespaces bool
		output        string
	}{
		"projects exist, full format": {
			projects:     testProjects(organization, "dev", "staging", "prod"),
			outputFormat: full,
			output: `NAME
dev
prod
staging
`,
		},
		"projects exist, no header format": {
			projects:     testProjects(organization, "dev", "staging", "prod"),
			outputFormat: noHeader,
			output: `dev
prod
staging
`,
		},
		"projects exist and all namespaces set": {
			projects:      testProjects(organization, "dev", "staging", "prod"),
			outputFormat:  full,
			allNamespaces: true,
			output: `NAME
dev
prod
staging
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
			projects:     testProjects(organization, "dev", "staging"),
			name:         "dev",
			outputFormat: full,
			output: `NAME
dev
`,
		},
		"specific project requested, but does not exist": {
			projects:     testProjects(organization, "staging"),
			name:         "dev",
			outputFormat: full,
			output:       "no Projects found\n",
		},
		"specific project requested, yaml output": {
			projects:     testProjects(organization, "dev", "staging"),
			name:         "dev",
			outputFormat: yamlOut,
			output:       "\x1b[96mapiVersion\x1b[0m:\x1b[92m management.nine.ch/v1alpha1\x1b[0m\n\x1b[92m\x1b[0m\x1b[96mkind\x1b[0m:\x1b[92m Project\x1b[0m\n\x1b[92m\x1b[0m\x1b[96mmetadata\x1b[0m:\x1b[96m\x1b[0m\n\x1b[96m  name\x1b[0m:\x1b[92m dev\x1b[0m\n\x1b[92m  \x1b[0m\x1b[96mnamespace\x1b[0m:\x1b[92m evilcorp\x1b[0m\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			testCase := testCase

			get := &Cmd{
				Output:        testCase.outputFormat,
				AllNamespaces: testCase.allNamespaces,
			}

			scheme, err := api.NewScheme()
			if err != nil {
				t.Fatal(err)
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithIndex(&management.Project{}, "metadata.name", func(o client.Object) []string {
					return []string{o.GetName()}
				}).
				WithObjects(testCase.projects...).Build()

			// we set the namespace in the client to show that it
			// doesn't affect projects listing
			apiClient := &api.Client{WithWatch: client, Namespace: "default"}
			kubeconfig, err := test.CreateTestKubeconfig(apiClient, organization)
			require.NoError(t, err)
			defer os.Remove(kubeconfig)

			buf := &bytes.Buffer{}
			cmd := projectCmd{
				out:  buf,
				Name: testCase.name,
			}

			if err := cmd.Run(ctx, apiClient, get); err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, testCase.output, buf.String())
		})
	}
}

func testProjects(organization string, names ...string) []client.Object {
	var projects []client.Object
	for _, name := range names {
		projects = append(projects, &management.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: organization,
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       management.ProjectKind,
				APIVersion: management.SchemeGroupVersion.String(),
			},
			Spec: management.ProjectSpec{},
		})
	}
	return projects
}
