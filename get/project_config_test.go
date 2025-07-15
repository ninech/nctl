package get

import (
	"bytes"
	"context"
	"testing"
	"time"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestProjectConfigs(t *testing.T) {
	ctx := context.Background()

	cases := map[string]struct {
		get                        *Cmd
		project                    string
		createdConfigs             []client.Object
		expectExactMessage         *string
		expectedLineAmountInOutput *int
	}{
		"get configs for all projects": {
			get: &Cmd{
				Output:      full,
				AllProjects: true,
			},
			project: "ns-1",
			createdConfigs: []client.Object{
				fakeProjectConfig(time.Second*10, "ns-1", "ns-1"),
				fakeProjectConfig(time.Second*12, "ns-2", "ns-2"),
				fakeProjectConfig(time.Second*13, "ns-3", "ns-3"),
			},
			// we expect the header line and 3 project configs
			expectedLineAmountInOutput: ptr.To(4),
		},
		"get config for current project": {
			get: &Cmd{
				Output: full,
			},
			project: "ns-2",
			createdConfigs: []client.Object{
				fakeProjectConfig(time.Second*10, "ns-2", "ns-2"),
				// we are creating a config in a
				// different project, to test if it is
				// excluded from the output
				fakeProjectConfig(time.Second*10, "ns-3", "ns-3"),
			},
			// header line + 1 project config
			expectedLineAmountInOutput: ptr.To(2),
		},
		"no configs existing": {
			get: &Cmd{
				Output: full,
			},
			project:            "ns-3",
			expectExactMessage: ptr.To("no ProjectConfigs found in project ns-3\n"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			apiClient, err := test.SetupClient(
				test.WithProjectsFromResources(tc.createdConfigs...),
				test.WithObjects(tc.createdConfigs...),
				test.WithKubeconfig(t),
				test.WithDefaultProject(tc.project),
				test.WithNameIndexFor(&apps.ProjectConfig{}),
			)
			require.NoError(t, err)

			buf := &bytes.Buffer{}
			cmd := configsCmd{}
			cmd.out = buf

			if err := cmd.Run(ctx, apiClient, tc.get); err != nil {
				t.Fatal(err)
			}
			if tc.expectedLineAmountInOutput != nil {
				assert.Equal(t, *tc.expectedLineAmountInOutput, test.CountLines(buf.String()), buf.String())
			}

			if tc.expectExactMessage == nil {
				return
			}
			assert.Equal(t, buf.String(), *tc.expectExactMessage, buf.String())
		})
	}
}

func fakeProjectConfig(
	creationTimeOffset time.Duration,
	name, namespace string,
) *apps.ProjectConfig {
	return &apps.ProjectConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
			CreationTimestamp: metav1.NewTime(defaultCreationTime.Add(creationTimeOffset)),
		},
		Spec: apps.ProjectConfigSpec{
			ForProvider: apps.ProjectConfigParameters{
				Config: apps.Config{
					Size:     test.AppMicro,
					Replicas: ptr.To(int32(1)),
					Port:     ptr.To(int32(9000)),
					Env:      util.EnvVarsFromMap(map[string]string{"key1": "val1"}),
				},
			},
		},
	}
}
