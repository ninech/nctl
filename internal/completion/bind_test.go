package completion

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	kongcompletion "github.com/jotaen/kong-completion"
	"github.com/posener/complete"
	"github.com/stretchr/testify/require"
)

// bindGrammar mirrors the parts of the CLI the resource name predictors depend
// on: a global project flag and commands naming their API resource through
// their own name, an alias or the api-resource tag.
type bindGrammar struct {
	Project string `help:"Limit commands to a specific project." short:"p"`
	Get     struct {
		Postgres struct {
			Name string `arg:"" completion-predictor:"client:resource_name" default:""`
		} `cmd:"" aliases:"pg"`
		Clusters struct {
			Name string `arg:"" completion-predictor:"client:resource_name" default:""`
		} `cmd:"" aliases:"vcluster" api-resource:"kubernetesclusters"`
	} `cmd:""`
	Delete struct {
		Postgres struct {
			Name string `arg:"" completion-predictor:"client:resource_name" default:""`
		} `cmd:""`
	} `cmd:""`
}

func TestBindResourceNamesCompletesTheCommand(t *testing.T) {
	tests := []struct {
		name     string
		compLine string
		want     string
	}{
		{
			name:     "command without flags",
			compLine: "nctl get postgres ",
			want:     "postgres",
		},
		{
			name:     "trailing short project flag",
			compLine: "nctl get postgres -p myproject ",
			want:     "postgres",
		},
		{
			name:     "trailing long project flag",
			compLine: "nctl get postgres --project myproject ",
			want:     "postgres",
		},
		{
			name:     "trailing assigned project flag",
			compLine: "nctl get postgres --project=myproject ",
			want:     "postgres",
		},
		{
			name:     "project flag before the command",
			compLine: "nctl -p myproject get postgres ",
			want:     "postgres",
		},
		{
			name:     "partially typed resource name",
			compLine: "nctl get postgres -p myproject bound",
			want:     "postgres",
		},
		{
			name:     "command alias",
			compLine: "nctl get pg -p myproject ",
			want:     "postgres",
		},
		{
			name:     "command declaring its resource",
			compLine: "nctl get clusters -p myproject ",
			want:     "kubernetesclusters",
		},
		{
			name:     "alias of a command declaring its resource",
			compLine: "nctl get vcluster ",
			want:     "kubernetesclusters",
		},
		{
			name:     "resource of another parent command",
			compLine: "nctl delete postgres -p myproject ",
			want:     "postgres",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := require.New(t)

			cmd := newBoundTestCommand(t)

			t.Setenv("COMP_LINE", tt.compLine)
			t.Setenv("COMP_POINT", strconv.Itoa(len(tt.compLine)))

			out := &bytes.Buffer{}
			completer := complete.New("nctl", cmd)
			completer.Out = out
			is.True(completer.Complete())

			is.Equal([]string{"bound:" + tt.want}, strings.Fields(out.String()))
		})
	}
}

func TestBindResourceNamesLeavesOtherPredictors(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	var grammar struct {
		Get struct {
			Postgres struct {
				Name string `arg:"" completion-predictor:"other" default:""`
			} `cmd:""`
		} `cmd:""`
	}

	parser, err := kong.New(&grammar)
	is.NoError(err)

	other := complete.PredictSet("untouched")
	cmd, err := kongcompletion.Command(parser, kongcompletion.WithPredictor("other", other))
	is.NoError(err)

	is.NoError(bindResourceNames(cmd, parser.Model.Node, boundTo))

	predictors, ok := cmd.Sub["get"].Sub["postgres"].Args.(*kongcompletion.PositionalPredictor)
	is.True(ok)
	is.Equal([]string{"untouched"}, predictors.Predictors[0].Predict(complete.Args{}))
}

func TestBindResourceNamesWithoutNode(t *testing.T) {
	t.Parallel()

	require.NoError(t, bindResourceNames(complete.Command{}, nil, boundTo))
}

func TestUnboundResourceNamePredictsNothing(t *testing.T) {
	t.Parallel()

	require.Nil(t, unboundResourceName{}.Predict(complete.Args{}))
}

// newBoundTestCommand builds the completion tree of [bindGrammar] with every
// resource name predictor bound to a marker naming its resource.
func newBoundTestCommand(t *testing.T) complete.Command {
	t.Helper()
	is := require.New(t)

	parser, err := kong.New(&bindGrammar{})
	is.NoError(err)

	cmd, err := kongcompletion.Command(parser,
		kongcompletion.WithPredictor(ResourceName, unboundResourceName{}))
	is.NoError(err)

	is.NoError(bindResourceNames(cmd, parser.Model.Node, boundTo))

	return cmd
}

func boundTo(resource string) complete.Predictor {
	return complete.PredictSet("bound:" + resource)
}

// argFlagGrammar mirrors the parts of the CLI the database predictors depend on:
// global flags taking a value and a boolean one, in front of a command naming its instance positionally.
type argFlagGrammar struct {
	Project string `help:"Limit commands to a specific project." short:"p"`
	Verbose bool   `help:"Show verbose messages."`
	Exec    struct {
		MySQL struct {
			Name     string `arg:"" default:""`
			Database string `short:"d" completion-predictor:"spy"`
		} `cmd:"" name:"mysql"`
	} `cmd:""`
}

// argFlagSpy reports the instance name it reads off the command line with the flags [bindArgFlags] bound to it.
type argFlagSpy struct {
	argFlags []string
}

func (s argFlagSpy) Predict(args complete.Args) []string {
	if name := firstPositionalArg(args.Completed, s.argFlags); name != "" {
		return []string{"found:" + name}
	}

	return []string{"found:none"}
}

func (s argFlagSpy) withArgFlags(argFlags []string) complete.Predictor {
	s.argFlags = argFlags

	return s
}

func TestBindArgFlagsFindsThePositionalArg(t *testing.T) {
	tests := []struct {
		name     string
		compLine string
		want     string
	}{
		{
			name:     "instance name only",
			compLine: "nctl exec mysql myinstance -d ",
			want:     "found:myinstance",
		},
		{
			name:     "boolean flag before the instance name",
			compLine: "nctl exec mysql --verbose myinstance -d ",
			want:     "found:myinstance",
		},
		{
			name:     "value flag before the instance name",
			compLine: "nctl exec mysql -p myproject myinstance -d ",
			want:     "found:myinstance",
		},
		{
			name:     "both flag kinds before the instance name",
			compLine: "nctl exec mysql --verbose --project myproject myinstance -d ",
			want:     "found:myinstance",
		},
		{
			name:     "no instance name",
			compLine: "nctl exec mysql --verbose -d ",
			want:     "found:none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := require.New(t)

			parser, err := kong.New(&argFlagGrammar{})
			is.NoError(err)

			cmd, err := kongcompletion.Command(parser, kongcompletion.WithPredictor("spy", argFlagSpy{}))
			is.NoError(err)
			is.NoError(bindArgFlags(cmd, parser.Model.Node))

			t.Setenv("COMP_LINE", tt.compLine)
			t.Setenv("COMP_POINT", strconv.Itoa(len(tt.compLine)))

			out := &bytes.Buffer{}
			completer := complete.New("nctl", cmd)
			completer.Out = out
			is.True(completer.Complete())

			is.Equal([]string{tt.want}, strings.Fields(out.String()))
		})
	}
}

func TestBindArgFlagsWithoutNode(t *testing.T) {
	t.Parallel()

	require.NoError(t, bindArgFlags(complete.Command{}, nil))
}

func TestBindArgFlagsLeavesOtherPredictors(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	parser, err := kong.New(&argFlagGrammar{})
	is.NoError(err)

	other := complete.PredictSet("untouched")
	cmd, err := kongcompletion.Command(parser, kongcompletion.WithPredictor("spy", other))
	is.NoError(err)
	is.NoError(bindArgFlags(cmd, parser.Model.Node))

	is.Equal([]string{"untouched"},
		cmd.Sub["exec"].Sub["mysql"].GlobalFlags["--database"].Predict(complete.Args{}))
}
