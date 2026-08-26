package edit

import (
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	apps "github.com/ninech/apis/apps/v1alpha1"
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/apiresource"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func TestEdit(t *testing.T) {
	// set cat as our "editor" for testing
	for _, env := range editorEnvs {
		t.Setenv(env, "cat")
	}
	originalStdOut := os.Stdout

	tests := map[string]struct {
		command             string
		resource            client.Object
		sourceGitAuthSecret *corev1.Secret
		staticEgress        *networking.StaticEgress
		cmd                 resourceCmd
		expectedErr         string
	}{
		"command named after the resource": {
			command: "application",
			resource: &apps.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
			},
			cmd: resourceCmd{
				Name: "app",
			},
		},
		"command hyphenating the name of the resource": {
			command: "project-config",
			resource: &apps.ProjectConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "cfg", Namespace: "default"},
			},
			cmd: resourceCmd{
				Name: "cfg",
			},
		},
		"alias of a command hyphenating the name of the resource": {
			command: "config",
			resource: &apps.ProjectConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "cfg", Namespace: "default"},
			},
			cmd: resourceCmd{
				Name: "cfg",
			},
		},
		"command declaring the resource it acts on": {
			command: "vcluster",
			resource: &infrastructure.KubernetesCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster", Namespace: "default"},
			},
			cmd: resourceCmd{
				Name: "cluster",
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			is := require.New(t)

			objs := []client.Object{tc.resource}
			apiClient := test.SetupClient(t, test.WithObjects(objs...))
			r, w, _ := os.Pipe()
			os.Stdout = w
			err := tc.cmd.Run(selectCommand(t, tc.command, tc.cmd.Name), t.Context(), apiClient)
			w.Close()
			os.Stdout = originalStdOut
			if tc.expectedErr != "" {
				is.ErrorContains(err, tc.expectedErr)
				return
			}
			is.NoError(err)
			out, err := io.ReadAll(r)
			is.NoError(err)

			gvk, err := apiutil.GVKForObject(tc.resource, apiClient.Scheme())
			is.NoError(err)
			tc.resource.GetObjectKind().SetGroupVersionKind(gvk)
			is.True(strings.HasPrefix(string(out), fmt.Sprintf(header, formatObj(tc.resource))), "header matches")
		})
	}
}

// selectCommand parses command using Cmd grammar, returning a kong.Context with declared tags.
func selectCommand(t *testing.T, command, name string) *kong.Context {
	t.Helper()

	parser, err := kong.New(&Cmd{}, kong.BindTo(io.Discard, (*io.Writer)(nil)))
	require.NoError(t, err)

	kctx, err := parser.Parse([]string{command, name})
	require.NoError(t, err)

	return kctx
}

func TestEditCommandsResolve(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	scheme, err := api.NewScheme()
	is.NoError(err)

	parser, err := kong.New(&Cmd{}, kong.BindTo(io.Discard, (*io.Writer)(nil)))
	is.NoError(err)

	for _, node := range parser.Model.Children {
		if node.Type != kong.CommandNode {
			continue
		}
		_, err := apiresource.FindKind(scheme, apiresource.OfCommand(node))
		is.NoError(err, "edit %s edits no known resource", node.Name)
	}
}
