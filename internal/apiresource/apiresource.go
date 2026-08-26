// Package apiresource resolves API resources and GVKs associated with CLI commands.
package apiresource

import (
	"fmt"
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
	if resource != "" {
		for gvk := range scheme.AllKnownTypes() {
			if strings.HasSuffix(gvk.Kind, listSuffix) ||
				!strings.HasSuffix(strings.ToLower(gvk.Group), groupSuffix) {
				continue
			}
			if normalize(gvk.Kind) == normalize(resource) {
				return gvk, nil
			}
		}
	}

	return schema.GroupVersionKind{}, fmt.Errorf("no API resource named %q", resource)
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

func normalize(s string) string {
	return flect.Pluralize(strings.ToLower(strings.ReplaceAll(s, "-", "")))
}
