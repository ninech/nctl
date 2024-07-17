package util

import (
	"sort"

	apps "github.com/ninech/apis/apps/v1alpha1"
)

// OrderReleaseList orders the given list of releases, moving the latest
// release to the beginning of the list
func OrderReleaseList(releaseList *apps.ReleaseList) {
	if len(releaseList.Items) <= 1 {
		return
	}

	sort.Slice(releaseList.Items, func(i, j int) bool {
		applicationNameI := releaseList.Items[i].ObjectMeta.Labels[ApplicationNameLabel]
		applicationNameJ := releaseList.Items[j].ObjectMeta.Labels[ApplicationNameLabel]

		if applicationNameI != applicationNameJ {
			return applicationNameI < applicationNameJ
		}

		return releaseList.Items[i].CreationTimestampNano < releaseList.Items[j].CreationTimestampNano
	})
}
