// Package apiresource resolves API resources and GVKs associated with CLI commands.
package apiresource

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/gobuffalo/flect"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Tag is the struct tag used when a command name differs from its target API resource
// (e.g. "clusters" acting on "kubernetesclusters").
const Tag = "api-resource"

const (
	listSuffix  = "List"
	groupSuffix = "nine.ch"
)

// OfCommand returns the API resource name declared in the node tag, or the node name if unset.
func OfCommand(node *kong.Node) string {
	if node == nil {
		return ""
	}
	if node.Tag != nil {
		if resource := node.Tag.Get(Tag); resource != "" {
			return resource
		}
	}

	return node.Name
}

// FindKind resolves a resource name (singular, plural, or hyphenated) to its Nine API [schema.GroupVersionKind].
func FindKind(scheme *runtime.Scheme, resource string) (schema.GroupVersionKind, error) {
	var found bool
	var match schema.GroupVersionKind

	if resource != "" {
		for gvk := range scheme.AllKnownTypes() {
			if strings.HasSuffix(gvk.Kind, listSuffix) ||
				!strings.HasSuffix(strings.ToLower(gvk.Group), groupSuffix) {
				continue
			}
			if normalize(gvk.Kind) != normalize(resource) {
				continue
			}
			if !found || compareKinds(scheme, gvk, match) < 0 {
				found, match = true, gvk
			}
		}
	}

	if !found {
		return schema.GroupVersionKind{}, fmt.Errorf("no API resource named %q", resource)
	}

	return match, nil
}

// FindListKind resolves a resource name to its list [schema.GroupVersionKind].
func FindListKind(scheme *runtime.Scheme, resource string) (schema.GroupVersionKind, error) {
	gvk, err := FindKind(scheme, resource)
	if err != nil {
		return schema.GroupVersionKind{}, err
	}

	list := gvk.GroupVersion().WithKind(gvk.Kind + listSuffix)
	if !scheme.Recognizes(list) {
		return schema.GroupVersionKind{}, fmt.Errorf("the API resource %q has no kind %q", resource, list.Kind)
	}

	return list, nil
}

// compareKinds orders two kinds.
func compareKinds(scheme *runtime.Scheme, a, b schema.GroupVersionKind) int {
	return cmp.Or(
		cmp.Compare(a.Group, b.Group),
		cmp.Compare(versionPriority(scheme, a.GroupVersion()), versionPriority(scheme, b.GroupVersion())),
		cmp.Compare(a.Kind, b.Kind),
	)
}

// versionPriority returns the position of gv within the versions the scheme prioritizes for its group.
func versionPriority(scheme *runtime.Scheme, gv schema.GroupVersion) int {
	versions := scheme.PrioritizedVersionsForGroup(gv.Group)
	if i := slices.Index(versions, gv); i >= 0 {
		return i
	}

	return len(versions)
}

func normalize(s string) string {
	return flect.Pluralize(strings.ToLower(strings.ReplaceAll(s, "-", "")))
}
