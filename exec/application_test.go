package exec

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	project = test.DefaultProject
)

func TestApplicationReplicaSelection(t *testing.T) {
	t.Parallel()

	const (
		firstApp, secondApp = "first-app", "second-app"
	)

	for name, testCase := range map[string]struct {
		application string
		// releases will get an automatic timestamp added. The first
		// release in the slice will be the oldest release.
		releases          []apps.Release
		expectedReplica   string
		expectedBuildType appBuildType
		expectError       bool
	}{
		"happy-path-single-release": {
			application: firstApp,
			releases: []apps.Release{
				newRelease(
					firstApp,
					[]apps.ReplicaObservation{
						{
							Status:      apps.ReplicaStatusReady,
							ReplicaName: "test-replica-1",
						},
					},
					apps.ReleaseProcessStatusAvailable,
					false,
				),
			},
			expectedReplica:   "test-replica-1",
			expectedBuildType: appBuildTypeBuildpack,
		},
		"happy-path-single-release-multiple-replicas": {
			application: firstApp,
			releases: []apps.Release{
				newRelease(
					firstApp,
					[]apps.ReplicaObservation{
						{
							Status:      apps.ReplicaStatusReady,
							ReplicaName: "test-replica-1",
						},
						{
							Status:      apps.ReplicaStatusReady,
							ReplicaName: "test-replica-2",
						},
						{
							Status:      apps.ReplicaStatusReady,
							ReplicaName: "test-replica-3",
						},
					},
					apps.ReleaseProcessStatusAvailable,
					false,
				),
			},
			// we make sure that we always take the first replica
			// even if multiple ready ones are available
			expectedReplica:   "test-replica-1",
			expectedBuildType: appBuildTypeBuildpack,
		},
		"happy-path-multiple-releases": {
			application: firstApp,
			releases: []apps.Release{
				newRelease(
					firstApp,
					[]apps.ReplicaObservation{
						{
							Status:      apps.ReplicaStatusReady,
							ReplicaName: "test-replica-1",
						},
					},
					apps.ReleaseProcessStatusSuperseded,
					false,
				),
				newRelease(
					firstApp,
					[]apps.ReplicaObservation{
						{
							Status:      apps.ReplicaStatusReady,
							ReplicaName: "test-replica-2",
						},
					},
					apps.ReleaseProcessStatusAvailable,
					false,
				),
			},
			expectedReplica:   "test-replica-2",
			expectedBuildType: appBuildTypeBuildpack,
		},
		"happy-path-multiple-releases-with-failing-ones": {
			application: firstApp,
			releases: []apps.Release{
				newRelease(
					firstApp,
					[]apps.ReplicaObservation{
						{
							Status:      apps.ReplicaStatusReady,
							ReplicaName: "test-replica-1",
						},
					},
					apps.ReleaseProcessStatusAvailable,
					false,
				),
				newRelease(
					firstApp,
					[]apps.ReplicaObservation{
						{
							Status:      apps.ReplicaStatusFailing,
							ReplicaName: "test-replica-2",
						},
					},
					apps.ReleaseProcessStatusFailure,
					false,
				),
				newRelease(
					firstApp,
					[]apps.ReplicaObservation{
						{
							Status:      apps.ReplicaStatusFailing,
							ReplicaName: "test-replica-3",
						},
					},
					apps.ReleaseProcessStatusFailure,
					false,
				),
			},
			expectedReplica:   "test-replica-1",
			expectedBuildType: appBuildTypeBuildpack,
		},
		"happy-path-multiple-apps-and-releases": {
			application: firstApp,
			releases: []apps.Release{
				newRelease(
					firstApp,
					[]apps.ReplicaObservation{
						{
							Status:      apps.ReplicaStatusReady,
							ReplicaName: "test-replica-1",
						},
					},
					apps.ReleaseProcessStatusSuperseded,
					false,
				),
				newRelease(
					firstApp,
					[]apps.ReplicaObservation{
						{
							Status:      apps.ReplicaStatusReady,
							ReplicaName: "test-replica-2",
						},
					},
					apps.ReleaseProcessStatusAvailable,
					false,
				),
				newRelease(
					secondApp,
					[]apps.ReplicaObservation{
						{
							Status:      apps.ReplicaStatusReady,
							ReplicaName: "test-replica-3",
						},
					},
					apps.ReleaseProcessStatusAvailable,
					false,
				),
			},
			expectedReplica:   "test-replica-2",
			expectedBuildType: appBuildTypeBuildpack,
		},
		"no-release-available": {
			application: firstApp,
			releases:    []apps.Release{},
			expectError: true,
		},
		"only-progressing-release-available": {
			application: firstApp,
			releases: []apps.Release{
				newRelease(
					firstApp,
					[]apps.ReplicaObservation{
						{
							Status:      apps.ReplicaStatusProgressing,
							ReplicaName: "test-replica-1",
						},
					},
					apps.ReleaseProcessStatusProgressing,
					false,
				),
			},
			expectError: true,
		},
		"replica-is-suddenly-failing-in-available-release": {
			application: firstApp,
			releases: []apps.Release{
				newRelease(
					firstApp,
					[]apps.ReplicaObservation{
						{
							Status:      apps.ReplicaStatusFailing,
							ReplicaName: "test-replica-1",
						},
					},
					apps.ReleaseProcessStatusAvailable,
					false,
				),
			},
			expectError: true,
		},
		"selecting-ready-replica-amongst-multiple-ones-works": {
			application: firstApp,
			releases: []apps.Release{
				newRelease(
					firstApp,
					[]apps.ReplicaObservation{
						{
							Status:      apps.ReplicaStatusFailing,
							ReplicaName: "test-replica-1",
						},
						{
							Status:      apps.ReplicaStatusProgressing,
							ReplicaName: "test-replica-2",
						},
						{
							Status:      apps.ReplicaStatusReady,
							ReplicaName: "test-replica-3",
						},
					},
					apps.ReleaseProcessStatusAvailable,
					false,
				),
			},
			expectedReplica:   "test-replica-3",
			expectedBuildType: appBuildTypeBuildpack,
		},
		"dockerfile-builds-get-detected": {
			application: firstApp,
			releases: []apps.Release{
				newRelease(
					firstApp,
					[]apps.ReplicaObservation{
						{
							Status:      apps.ReplicaStatusReady,
							ReplicaName: "test-replica-1",
						},
					},
					apps.ReleaseProcessStatusAvailable,
					true,
				),
			},
			expectedReplica:   "test-replica-1",
			expectedBuildType: appBuildTypeDockerfile,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			apiClient := test.SetupClient(t,
				test.WithKubeconfig(),
				test.WithNameIndexFor(&apps.Release{}),
				test.WithObjects(addCreationTimestamp(testCase.releases)...),
				test.WithDefaultProject(project),
			)

			cmd := applicationCmd{resourceCmd: resourceCmd{Name: testCase.application}}
			replica, buildType, err := cmd.getReplica(t.Context(), apiClient)
			if testCase.expectError {
				is.Error(err)
				return
			}
			is.NoError(err)
			is.Equal(testCase.expectedReplica, replica)
			is.Equal(testCase.expectedBuildType, buildType)
		})
	}
}

func newRelease(
	appName string,
	replicaObservation []apps.ReplicaObservation,
	status apps.ReleaseProcessStatus,
	isDockerfileBuild bool,
) apps.Release {
	return apps.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("test-%s", uuid.New().String()),
			Namespace: project,
			Labels: map[string]string{
				util.ApplicationNameLabel: appName,
			},
		},
		Spec: apps.ReleaseSpec{
			ForProvider: apps.ReleaseParameters{
				DockerfileBuild: isDockerfileBuild,
				Build:           meta.LocalReference{Name: "test-build"},
				Image: meta.Image{
					Registry:   "https://some.registry",
					Repository: "some-repository",
					Tag:        "latest",
				},
			},
		},
		Status: apps.ReleaseStatus{
			AtProvider: apps.ReleaseObservation{
				ReleaseStatus:      status,
				ReplicaObservation: replicaObservation,
			},
		},
	}
}

// addCreationTimestamp adds a creation timestamp to each release with 1 second
// difference between each release. The last release in the slice will be the
// most current
func addCreationTimestamp(releases []apps.Release) []client.Object {
	baseTime := time.Now().Add(-1 * time.Hour)
	for i := range releases {
		releases[i].CreationTimestampNano = baseTime.Add(time.Duration(i) * time.Second).UnixNano()
	}
	objs := make([]client.Object, len(releases))
	for i, rel := range releases {
		objs[i] = &rel
	}

	return objs
}
