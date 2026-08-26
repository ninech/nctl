package completion

import (
	"strconv"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/ninech/nctl/api"
	"github.com/stretchr/testify/require"
)

func staticClient(c *api.Client) clientFunc {
	return func() (*api.Client, error) { return c, nil }
}

func TestAPICluster(t *testing.T) {
	tests := []struct {
		name     string
		compLine string
		env      string
		want     string
	}{
		{
			name:     "default without flag or environment",
			compLine: "nctl get applications ",
			want:     "nineapis.ch",
		},
		{
			name:     "environment overrides the default",
			compLine: "nctl get applications ",
			env:      "staging",
			want:     "staging",
		},
		{
			name:     "flag overrides the environment",
			compLine: "nctl --api-cluster dev get applications ",
			env:      "staging",
			want:     "dev",
		},
		{
			name:     "assigned flag overrides the environment",
			compLine: "nctl --api-cluster=dev get applications ",
			env:      "staging",
			want:     "dev",
		},
		{
			name:     "flag after the subcommand",
			compLine: "nctl get applications --api-cluster dev ",
			want:     "dev",
		},
		{
			name:     "incomplete flag falls back to the default",
			compLine: "nctl get applications --api-cluster",
			want:     "nineapis.ch",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("COMP_LINE", tt.compLine)
			t.Setenv("COMP_POINT", strconv.Itoa(len(tt.compLine)))
			t.Setenv("NCTL_API_CLUSTER", tt.env)

			require.Equal(t, tt.want, apiCluster(newAPIClusterParser(t)))
		})
	}
}

func TestAPIClusterWithoutFlag(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	var grammar struct {
		Project string
	}

	parser, err := kong.New(&grammar)
	is.NoError(err)

	is.Empty(apiCluster(parser))
	is.Empty(apiCluster(nil))
}

// newAPIClusterParser builds a parser declaring the API cluster flag the same
// way the CLI does, with the default coming from a Kong variable.
func newAPIClusterParser(t *testing.T) *kong.Kong {
	t.Helper()

	var grammar struct {
		APICluster string `help:"Context name of the API cluster." default:"${api_cluster}" env:"NCTL_API_CLUSTER" hidden:""`
		Get        struct {
			Applications struct{} `cmd:""`
		} `cmd:""`
	}

	parser, err := kong.New(&grammar, kong.Vars{"api_cluster": "nineapis.ch"})
	require.NoError(t, err)

	return parser
}
