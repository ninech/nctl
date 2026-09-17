// Package flag provides value types for kong flags which more than one command
// declares. Each type implements [encoding.TextUnmarshaler], which is how kong
// decodes a flag value into it.
package flag

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LabelSelector is a label query over a set of resources.
// https://pkg.go.dev/k8s.io/kubectl@v0.33.2/pkg/cmd/util#AddLabelSelectorFlagVar
type LabelSelector struct {
	metav1.LabelSelector
}

// UnmarshalText parses a label selector from a string.
// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#list-and-watch-filtering
func (ls *LabelSelector) UnmarshalText(text []byte) error {
	s := strings.TrimSpace(string(text))
	if s == "" {
		return nil
	}

	selector, err := metav1.ParseToLabelSelector(s)
	if err != nil {
		return fmt.Errorf("error parsing %q: %w", s, err)
	}
	ls.LabelSelector = *selector

	return nil
}
