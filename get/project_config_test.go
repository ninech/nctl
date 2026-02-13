package get

import (
	"bytes"
	"strings"
	"testing"
	"time"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/internal/application"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestProjectConfigs(t *testing.T) {
	cases := map[string]struct {
		get                        *Cmd
		project                    string
		createdConfigs             []client.Object
		expectExactMessage         *string
		expectedLineAmountInOutput *int
		errorContains              []string
	}{
		"get configs for all projects": {
			get: &Cmd{
				output: output{
					Format:      full,
					AllProjects: true,
				},
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
				output: output{
					Format: full,
				},
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
				output: output{
					Format: full,
				},
			},
			project:       "ns-3",
			errorContains: []string{`no "ProjectConfigs" found`, `Project: ns-3`},
		},
		"sensitive env var is masked": {
			get: &Cmd{
				output: output{
					Format: full,
				},
			},
			project: "ns-4",
			createdConfigs: []client.Object{
				&apps.ProjectConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ns-4",
						Namespace: "ns-4",
					},
					Spec: apps.ProjectConfigSpec{
						ForProvider: apps.ProjectConfigParameters{
							Config: apps.Config{
								Env: application.EnvVarsFromMap(map[string]string{"poo": "orange"}, application.Sensitive()),
							},
						},
					},
				},
			},
			expectExactMessage: ptr.To(
				"PROJECT  NAME  SIZE  REPLICAS  PORT  ENVIRONMENT_VARIABLES  BASIC_AUTH  DEPLOY_JOB  AGE\nns-4     ns-4                        poo=*****              false       <none>      292y\n",
			),
		},
		"non-sensitive env var is shown": {
			get: &Cmd{
				output: output{
					Format: full,
				},
			},
			project: "ns-5",
			createdConfigs: []client.Object{
				&apps.ProjectConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "ns-5",
						Namespace: "ns-5",
					},
					Spec: apps.ProjectConfigSpec{
						ForProvider: apps.ProjectConfigParameters{
							Config: apps.Config{
								Env: application.EnvVarsFromMap(map[string]string{"goo": "banana"}),
							},
						},
					},
				},
			},
			expectExactMessage: ptr.To(
				"PROJECT  NAME  SIZE  REPLICAS  PORT  ENVIRONMENT_VARIABLES  BASIC_AUTH  DEPLOY_JOB  AGE\nns-5     ns-5                        goo=banana             false       <none>      292y\n",
			),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			is := require.New(t)

			apiClient := test.SetupClient(t,
				test.WithProjectsFromResources(tc.createdConfigs...),
				test.WithObjects(tc.createdConfigs...),
				test.WithKubeconfig(),
				test.WithDefaultProject(tc.project),
				test.WithNameIndexFor(&apps.ProjectConfig{}),
			)

			buf := &bytes.Buffer{}
			tc.get.BeforeApply(buf)
			cmd := configsCmd{}

			err := cmd.Run(t.Context(), apiClient, tc.get)
			if len(tc.errorContains) > 0 {
				is.Error(err)
				for _, s := range tc.errorContains {
					is.Contains(strings.ToLower(err.Error()), strings.ToLower(s))
				}
				return
			}
			is.NoError(err)
			if tc.expectedLineAmountInOutput != nil {
				is.Equal(*tc.expectedLineAmountInOutput, test.CountLines(buf.String()), buf.String())
			}

			if tc.expectExactMessage == nil {
				return
			}
			is.Equal(*tc.expectExactMessage, buf.String(), buf.String())
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
					Env:      application.EnvVarsFromMap(map[string]string{"key1": "val1"}),
				},
			},
		},
	}
}
