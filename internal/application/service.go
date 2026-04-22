package application

import (
	"cmp"
	"slices"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/internal/format"
)

// ServiceMap is a map of service name to typed reference, used as a CLI flag type.
type ServiceMap map[string]TypedReference

// ServicesFromMap converts a map of name -> TypedReference into a
// NamedServiceTargetList. The namespace is set on each target.
func ServicesFromMap(services ServiceMap, namespace string) apps.NamedServiceTargetList {
	if len(services) == 0 {
		return nil
	}

	result := make(apps.NamedServiceTargetList, 0, len(services))
	for name, ref := range services {
		ref.Namespace = namespace
		result = append(result, apps.NamedServiceTarget{
			Name:   name,
			Target: ref.TypedReference,
		})
	}

	slices.SortFunc(result, func(a, b apps.NamedServiceTarget) int {
		return cmp.Compare(a.Name, b.Name)
	})

	return result
}

// UpdateServices merges toAdd into existing services (upsert by name) and
// removes services listed in toDelete. Warnings are emitted for delete-not-found cases.
func UpdateServices(existing apps.NamedServiceTargetList, toAdd apps.NamedServiceTargetList, toDelete []string, w format.Writer) apps.NamedServiceTargetList {
	// upsert: update existing or append new
	for _, add := range toAdd {
		found := false
		for i := range existing {
			if existing[i].Name == add.Name {
				existing[i].Target = add.Target
				found = true
				break
			}
		}
		if !found {
			existing = append(existing, add)
		}
	}

	// delete
	for _, name := range toDelete {
		before := len(existing)
		existing = slices.DeleteFunc(existing, func(s apps.NamedServiceTarget) bool {
			return s.Name == name
		})
		if len(existing) == before {
			w.Warningf("did not find a service with the name %q", name)
		}
	}

	return existing
}
