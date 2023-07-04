package get

import (
	"bytes"
	"context"
	"sort"
	"testing"
	"time"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestConfigs(t *testing.T) {
	scheme, err := api.NewScheme()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	cases := map[string]struct {
		cmd                configsCmd
		get                *Cmd
		project            string
		configs            *apps.ProjectConfigList
		otherConfigs       *apps.ProjectConfigList
		expectExactMessage string
	}{
		"get configs for all projects": {
			cmd: configsCmd{},
			get: &Cmd{
				Output:      full,
				AllProjects: true,
			},
			project: "ns-1",
			configs: &apps.ProjectConfigList{
				Items: []apps.ProjectConfig{
					*fakeProjectConfig(time.Second*10, "ns-1", "ns-1"),
					*fakeProjectConfig(time.Second*12, "ns-2", "ns-2"),
					*fakeProjectConfig(time.Second*13, "ns-3", "ns-3"),

					// As defined in the project documentation, a valid Project Configuration
					// has Name=Namespace. And this will be verified by a webhook.
					// However, for now, in `nctl` we don't have any logic to check for
					// this condition, so the following cases are also valid (here Name!=Namespace).
					*fakeProjectConfig(time.Second*10, "c1", "ns-1"),
					*fakeProjectConfig(time.Second*10, "c2", "ns-2"),
					*fakeProjectConfig(time.Second*13, "c3", "ns-3"),
				},
			},
			otherConfigs: &apps.ProjectConfigList{
				Items: []apps.ProjectConfig{},
			},
		},
		"get config for current project": {
			cmd: configsCmd{},
			get: &Cmd{
				Output: full,
			},
			project: "ns-2",
			configs: &apps.ProjectConfigList{
				Items: []apps.ProjectConfig{
					*fakeProjectConfig(time.Second*10, "ns-2", "ns-2"),
				},
			},
			otherConfigs: &apps.ProjectConfigList{
				Items: []apps.ProjectConfig{
					*fakeProjectConfig(time.Second*10, "c1", "ns-2"),
					*fakeProjectConfig(time.Second*13, "c3", "ns-2"),
				},
			},
		},
		"get a nonexistent config": {
			cmd: configsCmd{},
			get: &Cmd{
				Output: full,
			},
			project: "ns-3",
			configs: &apps.ProjectConfigList{
				Items: []apps.ProjectConfig{},
			},
			expectExactMessage: "no ProjectConfigs found in project ns-3\n",
			otherConfigs: &apps.ProjectConfigList{
				Items: []apps.ProjectConfig{
					*fakeProjectConfig(time.Second*10, "ns-1", "ns-1"),
					*fakeProjectConfig(time.Second*12, "ns-2", "ns-2"),

					*fakeProjectConfig(time.Second*10, "c1", "ns-3"),
					*fakeProjectConfig(time.Second*10, "c2", "ns-3"),
					*fakeProjectConfig(time.Second*13, "c3", "ns-3"),
				},
			},
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithIndex(&apps.ProjectConfig{}, "metadata.name", func(o client.Object) []string {
					return []string{o.GetName()}
				}).
				WithLists(tc.configs, tc.otherConfigs).
				Build()
			apiClient := &api.Client{WithWatch: client, Project: tc.project}

			configList := &apps.ProjectConfigList{}

			var opts []listOpt
			if !tc.get.AllProjects {
				opts = []listOpt{matchName(tc.project)}
			}

			if err := tc.get.list(ctx, apiClient, configList, opts...); err != nil {
				t.Fatal(err)
			}

			configOutputNames := []string{}
			for _, c := range tc.configs.Items {
				configOutputNames = append(configOutputNames, c.ObjectMeta.Name)
			}

			sort.Slice(configOutputNames, func(i, j int) bool {
				return configOutputNames[i] < configOutputNames[j]
			})

			configNames := []string{}
			for _, c := range tc.configs.Items {
				configNames = append(configNames, c.ObjectMeta.Name)
			}

			sort.Slice(configNames, func(i, j int) bool {
				return configNames[i] < configNames[j]
			})

			assert.Equal(t, configNames, configOutputNames)

			// At the time of writing this test, cmd.opts append is coupled with get.list().
			// Now, when we use get.list() with the same listOpt we duplicate entries.
			// I am creating new `get` here to avoid duplicates.
			tc.get.opts = nil

			tc.get.Output = noHeader

			buf := &bytes.Buffer{}
			tc.cmd.out = buf

			if err := tc.cmd.Run(ctx, apiClient, tc.get); err != nil {
				t.Fatal(err)
			}

			if tc.expectExactMessage != "" {
				assert.Equal(t, buf.String(), tc.expectExactMessage)
			} else {
				assert.Equal(t, len(tc.configs.Items), test.CountLines(buf.String()))
			}
			buf.Reset()
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
					Replicas: pointer.Int32(1),
					Port:     pointer.Int32(9000),
					Env:      util.EnvVarsFromMap(map[string]string{"key1": "val1"}),
				},
			},
		},
	}
}
