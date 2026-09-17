package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
)

func TestClusterName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		arg            string
		defaultProject string
		want           types.NamespacedName
		wantErr        string
	}{
		{
			name:           "name only uses the default project",
			arg:            "cluster",
			defaultProject: "proj",
			want:           types.NamespacedName{Name: "cluster", Namespace: "proj"},
		},
		{
			name:           "name/project overrides the default project",
			arg:            "cluster/other",
			defaultProject: "proj",
			want:           types.NamespacedName{Name: "cluster", Namespace: "other"},
		},
		{
			name: "name/project without a default project",
			arg:  "cluster/other",
			want: types.NamespacedName{Name: "cluster", Namespace: "other"},
		},
		{
			name:    "name only without a default project",
			arg:     "cluster",
			wantErr: "project cannot be empty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := clusterName(tt.arg, tt.defaultProject)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
