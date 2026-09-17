package serviceconnection

import (
	"io"
	"testing"

	"github.com/alecthomas/kong"
	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestKubernetesClusterOptionsAPIType parses real arguments through kong so
// that the flag names the embedded struct registers and the interpolated help
// are exercised along with the conversion to the API type.
func TestKubernetesClusterOptionsAPIType(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		args []string
		want *networking.KubernetesClusterOptions
	}{
		"none": {},
		"pod selector": {
			args: []string{"--source-pod-selector=app=web"},
			want: &networking.KubernetesClusterOptions{
				PodSelector: metav1.LabelSelector{
					MatchLabels:      map[string]string{"app": "web"},
					MatchExpressions: []metav1.LabelSelectorRequirement{},
				},
			},
		},
		"both selectors": {
			args: []string{"--source-pod-selector=app=web", "--source-namespace-selector=team in (a,b)"},
			want: &networking.KubernetesClusterOptions{
				PodSelector: metav1.LabelSelector{
					MatchLabels:      map[string]string{"app": "web"},
					MatchExpressions: []metav1.LabelSelectorRequirement{},
				},
				NamespaceSelector: metav1.LabelSelector{
					MatchLabels: map[string]string{},
					MatchExpressions: []metav1.LabelSelectorRequirement{
						{Key: "team", Operator: metav1.LabelSelectorOpIn, Values: []string{"a", "b"}},
					},
				},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			var cmd struct {
				KubernetesClusterOptions `embed:"" prefix:"source-"`
			}
			_, err := kong.Must(&cmd, KongVars(), kong.BindTo(io.Discard, (*io.Writer)(nil))).Parse(tt.args)
			is.NoError(err)

			is.Equal(tt.want, cmd.APIType())
		})
	}

	t.Run("nil receiver", func(t *testing.T) {
		t.Parallel()

		var kco *KubernetesClusterOptions
		require.Nil(t, kco.APIType())
	})
}
