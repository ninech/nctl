package get

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	defaultConfig = apps.Config{
		Size:     test.AppMicro,
		Replicas: ptr.To(int32(1)),
		Port:     ptr.To(int32(8080)),
	}

	defaultCreationTime = metav1.NewTime(test.MustParseTime(time.RFC3339, "2023-03-13T14:00:00Z"))
)

func TestReleases(t *testing.T) {
	const project = "some-project"

	scheme, err := api.NewScheme()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	cases := map[string]struct {
		cmd              releasesCmd
		releases         *apps.ReleaseList
		otherAppReleases *apps.ReleaseList
	}{
		"one app, one release, get all releases": {
			cmd: releasesCmd{
				ApplicationName: "app1",
			},
			releases: &apps.ReleaseList{
				Items: []apps.Release{
					newRelease(time.Second*10, 10, "a1", project, "app1", "pc", test.StatusAvailable),
				},
			},
			otherAppReleases: &apps.ReleaseList{
				Items: []apps.Release{
					newRelease(time.Second*13, 10, "o-a1", project, "other-app1", "pc", test.StatusSuperseded),
					newRelease(time.Second*10, 20, "o-b1", project, "other-app1", "pc", test.StatusSuperseded),
					newRelease(time.Second*11, 30, "o-c1", project, "other-app1", "pc", test.StatusAvailable),

					newRelease(time.Second*15, 10, "o-a2", project, "other-app2", "pc", test.StatusSuperseded),
					newRelease(time.Second*10, 20, "o-b2", project, "other-app2", "pc", test.StatusAvailable),
				},
			},
		},

		"one app, multiple releases, get all releases": {
			cmd: releasesCmd{
				ApplicationName: "app2",
			},
			releases: &apps.ReleaseList{
				Items: []apps.Release{
					newRelease(time.Second*16, 10, "a2", project, "app2", "pc", test.StatusSuperseded),
					newRelease(time.Second*12, 20, "b2", project, "app2", "pc", test.StatusSuperseded),
					newRelease(time.Second*17, 30, "c2", project, "app2", "pc", test.StatusAvailable),
				},
			},
			otherAppReleases: &apps.ReleaseList{
				Items: []apps.Release{
					newRelease(time.Second*10, 10, "o-a1", project, "other-app1", "pc", test.StatusAvailable),

					newRelease(time.Second*11, 10, "o-a2", project, "other-app2", "pc", test.StatusSuperseded),
					newRelease(time.Second*12, 20, "o-b2", project, "other-app2", "pc", test.StatusAvailable),
				},
			},
		},

		"one app, no releases, get all releases": {
			cmd: releasesCmd{
				ApplicationName: "app3",
			},
			releases: &apps.ReleaseList{
				Items: []apps.Release{},
			},
			otherAppReleases: &apps.ReleaseList{
				Items: []apps.Release{
					newRelease(time.Second*10, 10, "o-a1", project, "other-app1", "pc", test.StatusAvailable),

					newRelease(time.Second*11, 10, "o-a2", project, "other-app2", "pc", test.StatusSuperseded),
					newRelease(time.Second*12, 20, "o-b2", project, "other-app2", "pc", test.StatusAvailable),
				},
			},
		},

		"all apps, multiple releases, get all releases": {
			cmd: releasesCmd{
				ApplicationName: "",
			},
			releases: &apps.ReleaseList{
				Items: []apps.Release{
					newRelease(time.Second*12, 10, "a3", project, "app3", "pc", test.StatusSuperseded),
					newRelease(time.Second*11, 20, "b3", project, "app3", "pc", test.StatusSuperseded),
					newRelease(time.Second*10, 30, "c3", project, "app3", "pc", test.StatusAvailable),

					newRelease(time.Second*10, 10, "a4", project, "app4", "pc", test.StatusSuperseded),
					newRelease(time.Second*10, 20, "b4", project, "app4", "pc", test.StatusAvailable),

					newRelease(time.Second*15, 10, "a5", project, "app5", "pc", test.StatusSuperseded),
					newRelease(time.Second*10, 20, "b5", project, "app5", "pc", test.StatusSuperseded),
					newRelease(time.Second*16, 30, "c5", project, "app5", "pc", test.StatusSuperseded),
					newRelease(time.Second*11, 40, "d5", project, "app5", "pc", test.StatusAvailable),
				},
			},
			otherAppReleases: &apps.ReleaseList{
				Items: []apps.Release{},
			},
		},

		"all apps, multiple releases, get specific release": {
			cmd: releasesCmd{
				Name:            "a4",
				ApplicationName: "",
			},
			releases: &apps.ReleaseList{
				Items: []apps.Release{
					newRelease(time.Second*10, 10, "a4", project, "app4", "pc", test.StatusSuperseded),
				},
			},
			otherAppReleases: &apps.ReleaseList{
				Items: []apps.Release{
					newRelease(time.Second*12, 10, "a3", project, "app3", "pc", test.StatusSuperseded),
					newRelease(time.Second*11, 20, "b3", project, "app3", "pc", test.StatusSuperseded),
					newRelease(time.Second*10, 30, "c3", project, "app3", "pc", test.StatusAvailable),

					newRelease(time.Second*10, 20, "b4", project, "app4", "pc", test.StatusAvailable),

					newRelease(time.Second*15, 10, "a5", project, "app5", "pc", test.StatusSuperseded),
					newRelease(time.Second*10, 20, "b5", project, "app5", "pc", test.StatusSuperseded),
					newRelease(time.Second*16, 30, "c5", project, "app5", "pc", test.StatusSuperseded),
					newRelease(time.Second*11, 40, "d5", project, "app5", "pc", test.StatusAvailable),
				},
			},
		},

		"one apps, multiple releases, get specific release": {
			cmd: releasesCmd{
				Name:            "b5",
				ApplicationName: "app5",
			},
			releases: &apps.ReleaseList{
				Items: []apps.Release{
					newRelease(time.Second*10, 20, "b5", project, "app5", "pc", test.StatusSuperseded),
				},
			},
			otherAppReleases: &apps.ReleaseList{
				Items: []apps.Release{
					newRelease(time.Second*12, 10, "a3", project, "app3", "pc", test.StatusSuperseded),
					newRelease(time.Second*11, 20, "b3", project, "app3", "pc", test.StatusSuperseded),
					newRelease(time.Second*10, 30, "c3", project, "app3", "pc", test.StatusAvailable),

					newRelease(time.Second*10, 10, "a4", project, "app4", "pc", test.StatusSuperseded),
					newRelease(time.Second*10, 20, "b4", project, "app4", "pc", test.StatusAvailable),

					newRelease(time.Second*15, 10, "a5", project, "app5", "pc", test.StatusSuperseded),
					newRelease(time.Second*16, 30, "c5", project, "app5", "pc", test.StatusSuperseded),
					newRelease(time.Second*11, 40, "d5", project, "app5", "pc", test.StatusAvailable),
				},
			},
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			// we initilize client we objects in different order than CreationTimestampNano -
			// we sort them by CreationTimestamp which is quite "random" here.
			// This gives us a more realistic test sample and allows to test the prepare Releases output logic.
			releasesByCreationTime := copyAndSortRelesesByCreationTime(tc.releases)
			otherAppReleasesByCreationTime := copyAndSortRelesesByCreationTime(tc.otherAppReleases)

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithIndex(&apps.Release{}, "metadata.name", func(o client.Object) []string {
					return []string{o.GetName()}
				}).
				WithLists(releasesByCreationTime, otherAppReleasesByCreationTime).
				Build()
			apiClient := &api.Client{WithWatch: client, Project: project}

			get := &Cmd{
				Output: full,
			}
			releaseList := &apps.ReleaseList{}

			opts := []listOpt{matchName(tc.cmd.Name)}
			if len(tc.cmd.ApplicationName) != 0 {
				opts = append(opts, matchLabel(util.ApplicationNameLabel, tc.cmd.ApplicationName))
			}

			if err := get.list(ctx, apiClient, releaseList, opts...); err != nil {
				t.Fatal(err)
			}

			orderReleaseList(releaseList)
			releaseNames := []string{}
			for _, r := range releaseList.Items {
				releaseNames = append(releaseNames, r.ObjectMeta.Name)
			}

			expectedReleaseNames := []string{}
			for _, r := range tc.releases.Items {
				expectedReleaseNames = append(expectedReleaseNames, r.ObjectMeta.Name)
			}

			assert.Equal(t, expectedReleaseNames, releaseNames)

			// At the time of writing this test, cmd.opts append is coupled with get.list().
			// Now, when we use get.list() with the same listOpt we duplicate entries.
			// I am creating new `get` here to avoid duplicates.
			get = &Cmd{
				Output: full,
			}
			if err := tc.cmd.Run(ctx, apiClient, get); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func newRelease(
	creationTimeOffset time.Duration,
	creationTimeNanoOffset int64,
	name, project, appName, providerName string,
	releaseStatus apps.ReleaseProcessStatus,
) apps.Release {
	return apps.Release{
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
				Config: defaultConfig,

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

func copyAndSortRelesesByCreationTime(src *apps.ReleaseList) *apps.ReleaseList {
	dst := &apps.ReleaseList{
		Items: make([]apps.Release, len(src.Items)),
	}
	copy(dst.Items, src.Items)

	sort.Slice(dst.Items, func(i, j int) bool {
		return dst.Items[i].CreationTimestamp.Time.Before(dst.Items[j].CreationTimestamp.Time)
	})

	return dst
}
