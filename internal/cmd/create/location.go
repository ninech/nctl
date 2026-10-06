package create

import (
	"errors"
	"strings"

	meta "github.com/ninech/apis/meta/v1alpha1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

// availableLocations returns the locations the API server reported as accepting
// new resources after it denied a create. It returns nil for any other error.
func availableLocations(err error) []meta.LocationName {
	var status apierrors.APIStatus
	if !errors.As(err, &status) {
		return nil
	}

	details := status.Status().Details
	if details == nil {
		return nil
	}

	for _, cause := range details.Causes {
		if cause.Type != meta.CauseTypeLocationRestricted {
			continue
		}

		var locations []meta.LocationName
		for name := range strings.FieldsSeq(cause.Message) {
			locations = append(locations, meta.LocationName(name))
		}

		return locations
	}

	return nil
}
