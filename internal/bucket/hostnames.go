package bucket

import (
	"strings"
)

// PatchCustomHostnames applies add/clear/delete operations to a slice of
// BucketCustomHostname entries. It follows the common "patch" pattern:
//   - If clear is true, the result is an empty list regardless of input.
//   - Entries in add are parsed and replace any existing ones with the same host.
//   - Entries in remove identify hostnames to be removed.
func PatchCustomHostnames(base []string, clear bool, add []string, remove []string) ([]string, error) {
	var set stringSet
	if clear {
		set = make(stringSet)
	} else {
		set = setFromSliceTrimmed(base)
	}

	for _, h := range add {
		if h = strings.TrimSpace(h); h != "" {
			set.Add(h)
		}
	}
	for _, h := range remove {
		if h = strings.TrimSpace(h); h != "" {
			set.Del(h)
		}
	}

	return mapKeysSorted(set), nil
}
