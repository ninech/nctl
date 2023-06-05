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
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	// TODO: would be nice to get this from "github.com/ninech/apis/apps/v1alpha1"
	AppMicro     apps.ApplicationSize = "micro"
	AppMini      apps.ApplicationSize = "mini"
	AppStandard1 apps.ApplicationSize = "standard-1"

	StatusSuperseded apps.ReleaseProcessStatus = "superseded"
	StatusAvailable  apps.ReleaseProcessStatus = "available"

	defaultConfig = apps.Config{
		Size:     &AppMicro,
		Replicas: pointer.Int32(1),
		Port:     pointer.Int32(8080),
	}

	defaultCreationTime = metav1.NewTime(mustParseTime(time.RFC3339, "2023-03-13T14:00:00Z"))
)

func mustParseTime(format string, value string) time.Time {
	t, err := time.Parse(format, value)
	if err != nil {
		panic(err)
	}
	return t
}

func TestReleases(t *testing.T) {
	const namespace = "some-namespace"

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
					newRelease(time.Second*10, 10, "a1", namespace, "app1", "pc", StatusAvailable),
				},
			},
			otherAppReleases: &apps.ReleaseList{
				Items: []apps.Release{
					newRelease(time.Second*13, 10, "o-a1", namespace, "other-app1", "pc", StatusSuperseded),
					newRelease(time.Second*10, 20, "o-b1", namespace, "other-app1", "pc", StatusSuperseded),
					newRelease(time.Second*11, 30, "o-c1", namespace, "other-app1", "pc", StatusAvailable),

					newRelease(time.Second*15, 10, "o-a2", namespace, "other-app2", "pc", StatusSuperseded),
					newRelease(time.Second*10, 20, "o-b2", namespace, "other-app2", "pc", StatusAvailable),
				},
			},
		},

		"one app, multiple releases, get all releases": {
			cmd: releasesCmd{
				ApplicationName: "app2",
			},
			releases: &apps.ReleaseList{
				Items: []apps.Release{
					newRelease(time.Second*16, 10, "a2", namespace, "app2", "pc", StatusSuperseded),
					newRelease(time.Second*12, 20, "b2", namespace, "app2", "pc", StatusSuperseded),
					newRelease(time.Second*17, 30, "c2", namespace, "app2", "pc", StatusAvailable),
				},
			},
			otherAppReleases: &apps.ReleaseList{
				Items: []apps.Release{
					newRelease(time.Second*10, 10, "o-a1", namespace, "other-app1", "pc", StatusAvailable),

					newRelease(time.Second*11, 10, "o-a2", namespace, "other-app2", "pc", StatusSuperseded),
					newRelease(time.Second*12, 20, "o-b2", namespace, "other-app2", "pc", StatusAvailable),
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
					newRelease(time.Second*10, 10, "o-a1", namespace, "other-app1", "pc", StatusAvailable),

					newRelease(time.Second*11, 10, "o-a2", namespace, "other-app2", "pc", StatusSuperseded),
					newRelease(time.Second*12, 20, "o-b2", namespace, "other-app2", "pc", StatusAvailable),
				},
			},
		},

		"all apps, multiple releases, get all releases": {
			cmd: releasesCmd{
				ApplicationName: "",
			},
			releases: &apps.ReleaseList{
				Items: []apps.Release{
					newRelease(time.Second*12, 10, "a3", namespace, "app3", "pc", StatusSuperseded),
					newRelease(time.Second*11, 20, "b3", namespace, "app3", "pc", StatusSuperseded),
					newRelease(time.Second*10, 30, "c3", namespace, "app3", "pc", StatusAvailable),

					newRelease(time.Second*10, 10, "a4", namespace, "app4", "pc", StatusSuperseded),
					newRelease(time.Second*10, 20, "b4", namespace, "app4", "pc", StatusAvailable),

					newRelease(time.Second*15, 10, "a5", namespace, "app5", "pc", StatusSuperseded),
					newRelease(time.Second*10, 20, "b5", namespace, "app5", "pc", StatusSuperseded),
					newRelease(time.Second*16, 30, "c5", namespace, "app5", "pc", StatusSuperseded),
					newRelease(time.Second*11, 40, "d5", namespace, "app5", "pc", StatusAvailable),
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
					newRelease(time.Second*10, 10, "a4", namespace, "app4", "pc", StatusSuperseded),
				},
			},
			otherAppReleases: &apps.ReleaseList{
				Items: []apps.Release{
					newRelease(time.Second*12, 10, "a3", namespace, "app3", "pc", StatusSuperseded),
					newRelease(time.Second*11, 20, "b3", namespace, "app3", "pc", StatusSuperseded),
					newRelease(time.Second*10, 30, "c3", namespace, "app3", "pc", StatusAvailable),

					newRelease(time.Second*10, 20, "b4", namespace, "app4", "pc", StatusAvailable),

					newRelease(time.Second*15, 10, "a5", namespace, "app5", "pc", StatusSuperseded),
					newRelease(time.Second*10, 20, "b5", namespace, "app5", "pc", StatusSuperseded),
					newRelease(time.Second*16, 30, "c5", namespace, "app5", "pc", StatusSuperseded),
					newRelease(time.Second*11, 40, "d5", namespace, "app5", "pc", StatusAvailable),
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
					newRelease(time.Second*10, 20, "b5", namespace, "app5", "pc", StatusSuperseded),
				},
			},
			otherAppReleases: &apps.ReleaseList{
				Items: []apps.Release{
					newRelease(time.Second*12, 10, "a3", namespace, "app3", "pc", StatusSuperseded),
					newRelease(time.Second*11, 20, "b3", namespace, "app3", "pc", StatusSuperseded),
					newRelease(time.Second*10, 30, "c3", namespace, "app3", "pc", StatusAvailable),

					newRelease(time.Second*10, 10, "a4", namespace, "app4", "pc", StatusSuperseded),
					newRelease(time.Second*10, 20, "b4", namespace, "app4", "pc", StatusAvailable),

					newRelease(time.Second*15, 10, "a5", namespace, "app5", "pc", StatusSuperseded),
					newRelease(time.Second*16, 30, "c5", namespace, "app5", "pc", StatusSuperseded),
					newRelease(time.Second*11, 40, "d5", namespace, "app5", "pc", StatusAvailable),
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
			apiClient := &api.Client{WithWatch: client, Namespace: namespace}

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

			releaseOutputNames := make(map[string][]string)
			for appName, releaseOutput := range prepareReleasesOutput(releaseList.Items) {
				for _, r := range releaseOutput {
					releaseOutputNames[appName] =
						append(releaseOutputNames[appName], r.ObjectMeta.Name)
				}
			}

			releaseNames := make(map[string][]string)
			for _, release := range tc.releases.Items {
				releaseNames[release.ObjectMeta.Labels[util.ApplicationNameLabel]] =
					append(releaseNames[release.ObjectMeta.Labels[util.ApplicationNameLabel]], release.ObjectMeta.Name)
			}

			assert.Equal(t, releaseNames, releaseOutputNames)

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
	name, namespace, appName, providerName string,
	releaseStatus apps.ReleaseProcessStatus,
) apps.Release {
	return apps.Release{
		// we are using offset arg here - we need to make sure
		// that we have the desired order inside the Releases collection.
		CreationTimestampNano: defaultCreationTime.UnixNano() + creationTimeNanoOffset,
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         namespace,
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
					fmt.Sprintf("%s-%s-short.example.org", name, namespace),
					fmt.Sprintf("%s-%s-long.example.org", name, namespace),
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
