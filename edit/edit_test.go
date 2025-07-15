package edit

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	apps "github.com/ninech/apis/apps/v1alpha1"
	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func TestEdit(t *testing.T) {
	ctx := context.Background()
	// set cat as our "editor" for testing
	for _, env := range editorEnvs {
		t.Setenv(env, "cat")
	}
	originalStdOut := os.Stdout

	tests := map[string]struct {
		commandName         string
		commandAliases      []string
		resource            client.Object
		sourceGitAuthSecret *corev1.Secret
		staticEgress        *networking.StaticEgress
		cmd                 resourceCmd
		expectedErr         string
	}{
		"app": {
			commandName: "application",
			resource: &apps.Application{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
			},
			cmd: resourceCmd{
				Name: "app",
			},
		},
		"config": {
			commandName:    "config",
			commandAliases: []string{"projectconfig"},
			resource: &apps.ProjectConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "cfg", Namespace: "default"},
			},
			cmd: resourceCmd{
				Name: "cfg",
			},
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			objs := []client.Object{tc.resource}
			apiClient, err := test.SetupClient(test.WithObjects(objs...))
			require.NoError(t, err)
			r, w, _ := os.Pipe()
			os.Stdout = w
			err = tc.cmd.Run(&kong.Context{
				Path: []*kong.Path{
					{
						Command: &kong.Command{
							Name:    tc.commandName,
							Aliases: tc.commandAliases,
						},
					},
				},
			}, ctx, apiClient)
			w.Close()
			os.Stdout = originalStdOut
			if tc.expectedErr != "" {
				assert.ErrorContains(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)
			out, err := io.ReadAll(r)
			assert.NoError(t, err)

			gvk, err := apiutil.GVKForObject(tc.resource, apiClient.Scheme())
			require.NoError(t, err)
			tc.resource.GetObjectKind().SetGroupVersionKind(gvk)
			assert.True(t, strings.HasPrefix(string(out), fmt.Sprintf(header, formatObj(tc.resource))), "header matches")
		})
	}
}
