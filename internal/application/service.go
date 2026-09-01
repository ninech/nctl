package application

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/internal/format"
)

// NamedServiceReference is a named reference to a service target, in the
// form "name=kind/target-name".
type NamedServiceReference struct {
	Name   string
	Target TypedReference
}

// UnmarshalText parses a named service reference from a string in
// "name=kind/target-name" format.
func (n *NamedServiceReference) UnmarshalText(text []byte) error {
	name, rest, found := strings.Cut(string(text), "=")
	if !found || name == "" {
		return fmt.Errorf("unmarshal error: expected name=kind/target, got %q", text)
	}
	n.Name = name
	return n.Target.UnmarshalText([]byte(rest))
}

// ServicesFromReferences converts a slice of NamedServiceReference into a
// NamedServiceTargetList. The namespace is set on each target.
func ServicesFromReferences(services []NamedServiceReference, namespace string) apps.NamedServiceTargetList {
	if len(services) == 0 {
		return nil
	}

	result := make(apps.NamedServiceTargetList, 0, len(services))
	for _, ref := range services {
		target := ref.Target.TypedReference
		target.Namespace = namespace
		result = append(result, apps.NamedServiceTarget{
			Name:   ref.Name,
			Target: target,
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
			if existing[i].Name == add.Name && existing[i].Target.Kind == add.Target.Kind {
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
