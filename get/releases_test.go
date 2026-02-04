package get

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var defaultCreationTime = metav1.NewTime(test.MustParseTime(time.RFC3339, "2023-03-13T14:00:00Z"))

func TestReleases(t *testing.T) {
	const project = test.DefaultProject
	ctx := context.Background()

	cases := map[string]struct {
		cmd           releasesCmd
		releases      []client.Object
		inAllProjects bool
		// output will be full if not set
		output      outputFormat
		wantErr     bool
		wantContain []string
		wantLines   int
	}{
		"one app, one release, get all releases from specific app": {
			cmd: releasesCmd{
				ApplicationName: "app1",
			},
			releases: []client.Object{
				newRelease(time.Second*10, 10, "a1", project, "app1", "pc", test.StatusAvailable),
				// these releases are just added to show that
				// they do not influence the output
				newRelease(time.Second*13, 10, "o-a1", project, "other-app1", "pc", test.StatusSuperseded),
				newRelease(time.Second*10, 20, "o-b1", project, "other-app1", "pc", test.StatusSuperseded),
				newRelease(time.Second*11, 30, "o-c1", project, "other-app1", "pc", test.StatusAvailable),
			},
			wantContain: []string{"app1"},
			wantLines:   2, // header + result
		},

		"one app, multiple releases, get all releases from specific app": {
			cmd: releasesCmd{
				ApplicationName: "app2",
			},
			releases: []client.Object{
				newRelease(time.Second*16, 10, "a2", project, "app2", "pc", test.StatusSuperseded),
				newRelease(time.Second*12, 20, "b2", project, "app2", "pc", test.StatusSuperseded),
				newRelease(time.Second*17, 30, "c2", project, "app2", "pc", test.StatusAvailable),
				// another foreign release which should influence the output
				newRelease(time.Second*10, 10, "o-a1", project, "other-app1", "pc", test.StatusAvailable),
			},
			wantContain: []string{"a2", "b2", "c2"},
			wantLines:   4, // header + result
		},

		"one app, no releases, get all releases from specific app": {
			cmd: releasesCmd{
				ApplicationName: "app3",
			},
			releases: []client.Object{
				newRelease(time.Second*10, 10, "o-a1", project, "other-app1", "pc", test.StatusAvailable),
			},
			wantContain: []string{"no Releases found"},
			wantLines:   1,
		},

		"all apps, multiple releases, get all releases from all apps": {
			cmd: releasesCmd{},
			releases: []client.Object{
				// all app3 releases
				newRelease(time.Second*12, 10, "a3", project, "app3", "pc", test.StatusSuperseded),
				newRelease(time.Second*11, 20, "b3", project, "app3", "pc", test.StatusSuperseded),
				newRelease(time.Second*10, 30, "c3", project, "app3", "pc", test.StatusAvailable),

				// all app4 releases
				newRelease(time.Second*10, 10, "a4", project, "app4", "pc", test.StatusSuperseded),
				newRelease(time.Second*10, 20, "b4", project, "app4", "pc", test.StatusAvailable),
			},
			wantContain: []string{"a3", "b3", "c3", "a4", "b4"},
			wantLines:   6,
		},

		"all apps, multiple releases, get specific release": {
			cmd: releasesCmd{
				resourceCmd: resourceCmd{
					Name: "a4",
				},
			},
			releases: []client.Object{
				newRelease(time.Second*10, 10, "a4", project, "app4", "pc", test.StatusSuperseded),
				// this release should be ignored
				newRelease(time.Second*12, 10, "a3", project, "app3", "pc", test.StatusSuperseded),
			},
			wantContain: []string{"a4"},
			wantLines:   2,
		},

		"one app, multiple releases, get specific release from specific app": {
			cmd: releasesCmd{
				resourceCmd: resourceCmd{
					Name: "b5",
				},
				ApplicationName: "app5",
			},
			releases: []client.Object{
				newRelease(time.Second*10, 20, "b5", project, "app5", "pc", test.StatusSuperseded),
				newRelease(time.Second*12, 10, "a3", project, "app3", "pc", test.StatusSuperseded),
				newRelease(time.Second*10, 10, "a4", project, "app4", "pc", test.StatusSuperseded),
			},
			wantContain: []string{"b5", "app5"},
			wantLines:   2,
		},

		"list all releases in all projects": {
			cmd:           releasesCmd{},
			inAllProjects: true,
			releases: []client.Object{
				newRelease(time.Second*10, 20, "app1-release", project, "app1", "pc", test.StatusSuperseded),
				newRelease(time.Second*12, 10, "app2-release", "dev", "app2", "pc", test.StatusSuperseded),
				newRelease(time.Second*10, 10, "app3-release", "production", "app3", "pc", test.StatusSuperseded),
			},
			wantContain: []string{"app1-release", "app2-release", "app3-release"},
			wantLines:   4,
		},

		"list all releases of a specific app in all projects": {
			cmd: releasesCmd{
				ApplicationName: "app1",
			},
			inAllProjects: true,
			releases: []client.Object{
				newRelease(time.Second*10, 20, "falcon", project, "app1", "pc", test.StatusSuperseded),
				newRelease(time.Second*10, 20, "eagle", "dev", "app1", "pc", test.StatusSuperseded),
				newRelease(time.Second*12, 10, "starling", "production", "app2", "pc", test.StatusSuperseded),
			},
			wantContain: []string{"falcon", "eagle"},
			wantLines:   3,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			// we initialize the client with objects in different
			// order than CreationTimestampNano - we sort them by
			// CreationTimestamp which is quite "random" here.
			// This gives us a more realistic test sample and
			// allows to test the prepare Releases output logic.
			releasesByCreationTime := copyAndSortReleasesByCreationTime(tc.releases)

			apiClient, err := test.SetupClient(
				test.WithProjectsFromResources(releasesByCreationTime...),
				test.WithObjects(releasesByCreationTime...),
				test.WithNameIndexFor(&apps.Release{}),
				test.WithKubeconfig(t),
			)
			require.NoError(t, err)

			if tc.output == "" {
				tc.output = full
			}
			buf := &bytes.Buffer{}
			get := NewTestCmd(buf, tc.output)
			get.AllProjects = tc.inAllProjects

			if err := tc.cmd.Run(ctx, apiClient, get); (err != nil) != tc.wantErr {
				t.Errorf("releasesCmd.Run() error = %v, wantErr %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			for _, substr := range tc.wantContain {
				if !strings.Contains(buf.String(), substr) {
					t.Errorf("releasesCmd.Run() did not contain %q, out = %q", tc.wantContain, buf.String())
				}
			}
			if test.CountLines(buf.String()) != tc.wantLines {
				t.Errorf("expected the output to have %d lines, but found %d", tc.wantLines, test.CountLines(buf.String()))
				t.Log(buf.String())
			}
		})
	}
}

func newRelease(
	creationTimeOffset time.Duration,
	creationTimeNanoOffset int64,
	name, project, appName, providerName string,
	releaseStatus apps.ReleaseProcessStatus,
) *apps.Release {
	return &apps.Release{
		// we are using offset arg here - we need to make sure
		// that we have the desired order inside the Releases collection.
		CreationTimestampNano: defaultCreationTime.UnixNano() + creationTimeNanoOffset,
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         project,
			Labels:            map[string]string{util.ApplicationNameLabel: appName},
			CreationTimestamp: metav1.NewTime(defaultCreationTime.Add(creationTimeOffset)),
		},
		Spec: apps.ReleaseSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				ProviderConfigReference: &runtimev1.Reference{Name: providerName},
			},
			ForProvider: apps.ReleaseParameters{
				Build: meta.LocalReference{
					Name: "test",
				},
				Image: meta.Image{
					Repository: "nginx",
					Tag:        "stable-alpine",
				},
				Configuration: apps.Config{
					Size:     test.AppMicro,
					Replicas: ptr.To(int32(1)),
					Port:     ptr.To(int32(8080)),
				}.WithOrigin(apps.ConfigOriginApplication),

				// we always have at least 2 hosts here
				VerifiedHosts: []string{
					fmt.Sprintf("%s-%s-short.example.org", name, project),
					fmt.Sprintf("%s-%s-long.example.org", name, project),
				},
			},
		},
		Status: apps.ReleaseStatus{
			AtProvider: apps.ReleaseObservation{
				ReleaseStatus: releaseStatus,
			},
		},
	}
}

func copyAndSortReleasesByCreationTime(src []client.Object) []client.Object {
	newOrder := make([]client.Object, len(src))
	copy(newOrder, src)
	sort.Slice(newOrder, func(i, j int) bool {
		return newOrder[i].GetCreationTimestamp().Time.Before(newOrder[j].GetCreationTimestamp().Time)
	})

	return newOrder
}
