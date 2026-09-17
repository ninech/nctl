package flag

import (
	"fmt"
	"strings"

	meta "github.com/ninech/apis/meta/v1alpha1"
)

// LocalReference references another object in the same namespace.
type LocalReference struct {
	meta.LocalReference
}

// UnmarshalText parses a local reference from a string.
func (r *LocalReference) UnmarshalText(text []byte) error {
	name := strings.TrimSpace(string(text))
	if name == "" {
		return fmt.Errorf("reference unmarshal error: got %q", text)
	}

	r.Name = name

	return nil
}
