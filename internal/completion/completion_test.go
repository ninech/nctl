package completion

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	kongcompletion "github.com/jotaen/kong-completion"
	"github.com/stretchr/testify/require"
)

func TestCommandBindsResourceNames(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	parser, err := kong.New(&bindGrammar{})
	is.NoError(err)

	cmd, err := Command(t.Context(), parser)
	is.NoError(err)

	tests := map[string]string{
		"postgres": "postgres",
		"pg":       "postgres",
		"clusters": "kubernetesclusters",
		"vcluster": "kubernetesclusters",
	}

	for command, want := range tests {
		predictors, ok := cmd.Sub["get"].Sub[command].Args.(*kongcompletion.PositionalPredictor)
		is.True(ok, "get %s completes no positional argument", command)

		bound, ok := predictors.Predictors[0].(*resourcePredictor)
		is.True(ok, "get %s completes no resource name", command)
		is.Equal(want, bound.resource)
	}
}

func TestCommandWithoutParser(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	cmd, err := Command(t.Context(), nil)
	is.NoError(err)
	is.Empty(cmd.Sub)
}

func TestRegisterWithoutRequiredFlags(t *testing.T) {
	t.Parallel()

	var grammar struct {
		Project string `short:"p"`
	}

	parser, err := kong.New(&grammar)
	require.NoError(t, err)
	require.ErrorContains(t, Register(t.Context(), parser), apiClusterFlag)
}

func TestRegisterCompletes(t *testing.T) {
	is := require.New(t)

	const compLine = "nctl get "

	var grammar struct {
		APICluster string `name:"api-cluster"`
		Project    string `short:"p"`
		Get        struct {
			Postgres struct{} `cmd:""`
		} `cmd:""`
	}

	out := &bytes.Buffer{}
	exited := -1

	parser, err := kong.New(&grammar, kong.Name("nctl"), kong.Writers(out, out),
		kong.Exit(func(code int) { exited = code }))
	is.NoError(err)

	t.Setenv("COMP_LINE", compLine)
	t.Setenv("COMP_POINT", strconv.Itoa(len(compLine)))

	is.NoError(Register(t.Context(), parser))
	is.Equal(0, exited, "completion did not exit the CLI")
	is.Equal([]string{"postgres"}, strings.Fields(out.String()))
}

func TestRegisterWithoutCompletionLine(t *testing.T) {
	is := require.New(t)

	var grammar struct {
		APICluster string `name:"api-cluster"`
		Project    string `short:"p"`
		Get        struct {
			Postgres struct{} `cmd:""`
		} `cmd:""`
	}

	exited := -1

	parser, err := kong.New(&grammar, kong.Name("nctl"),
		kong.Exit(func(code int) { exited = code }))
	is.NoError(err)

	t.Setenv("COMP_LINE", "")

	is.NoError(Register(t.Context(), parser))
	is.Equal(-1, exited, "completion exited the CLI")
}
