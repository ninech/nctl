package application

import (
	"sort"

	apps "github.com/ninech/apis/apps/v1alpha1"
)

// OrderReleaseList orders the given list of releases first by name and then by
// creation timestamp latest to oldest. Reverse reverses the order by creation
// timestamp to oldest to latest.
func OrderReleaseList(releaseList *apps.ReleaseList, reverse bool) {
	if len(releaseList.Items) <= 1 {
		return
	}

	sort.Slice(releaseList.Items, func(i, j int) bool {
		applicationNameI := releaseList.Items[i].Labels[ApplicationNameLabel]
		applicationNameJ := releaseList.Items[j].Labels[ApplicationNameLabel]

		if applicationNameI != applicationNameJ {
			return applicationNameI < applicationNameJ
		}

		if reverse {
			return releaseList.Items[i].CreationTimestampNano < releaseList.Items[j].CreationTimestampNano
		}
		return releaseList.Items[i].CreationTimestampNano > releaseList.Items[j].CreationTimestampNano
	})
}
