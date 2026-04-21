package application

import (
	"fmt"
	"strings"

	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/cli"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// TypedReference is a reference to a resource with a specific type.
// It implements encoding.TextUnmarshaler to parse "kind/name" strings,
// which allows it to be used directly as a Kong flag type.
type TypedReference struct {
	meta.TypedReference
}

// UnmarshalText parses a typed reference from a string in "kind/name" format.
func (r *TypedReference) UnmarshalText(text []byte) error {
	s := strings.TrimSpace(string(text))
	kind, name, found := strings.Cut(s, "/")
	if !found || kind == "" || name == "" {
		return fmt.Errorf("unmarshal error: expected kind/name, got %q", text)
	}

	gvk, err := GroupVersionKindFromKind(kind)
	if err != nil {
		return fmt.Errorf("unmarshal error: %w", err)
	}

	r.Name = name
	r.GroupKind = metav1.GroupKind(gvk.GroupKind())

	return nil
}

// GroupVersionKindFromKind resolves a case-insensitive kind string to a
// schema.GroupVersionKind using the registered scheme.
func GroupVersionKindFromKind(kind string) (schema.GroupVersionKind, error) {
	scheme, err := api.NewScheme()
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("error creating scheme: %w", err)
	}

	for gvk := range scheme.AllKnownTypes() {
		if strings.EqualFold(kind, gvk.Kind) {
			return gvk, nil
		}
	}

	return schema.GroupVersionKind{}, cli.ErrorWithContext(fmt.Errorf("kind %q is invalid", kind)).
		WithExitCode(cli.ExitUsageError)
}
