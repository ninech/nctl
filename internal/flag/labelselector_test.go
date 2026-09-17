package flag

import (
	"testing"

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLabelSelector_UnmarshalText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		arg     string
		want    metav1.LabelSelector
		wantErr bool
	}{
		{"none", "", metav1.LabelSelector{MatchLabels: nil, MatchExpressions: nil}, false},
		{"simple", "key1=value1", metav1.LabelSelector{MatchLabels: map[string]string{"key1": "value1"}, MatchExpressions: []metav1.LabelSelectorRequirement{}}, false},
		{"surrounding whitespace", " key1=value1 ", metav1.LabelSelector{MatchLabels: map[string]string{"key1": "value1"}, MatchExpressions: []metav1.LabelSelectorRequirement{}}, false},
		{
			"expression", "key2 in (a,b)",
			metav1.LabelSelector{
				MatchLabels: map[string]string{},
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{Key: "key2", Operator: metav1.LabelSelectorOpIn, Values: []string{"a", "b"}},
				},
			},
			false,
		},
		{"invalid", "key1=value1,", metav1.LabelSelector{}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			ls := &LabelSelector{}
			err := ls.UnmarshalText([]byte(tt.arg))
			if tt.wantErr {
				is.Error(err)
				return
			}

			is.NoError(err)
			is.Equal(tt.want, ls.LabelSelector)
		})
	}
}
