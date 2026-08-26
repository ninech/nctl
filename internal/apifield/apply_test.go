package apifield

import (
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/posener/complete"
	"github.com/stretchr/testify/require"
)

func TestApplyHelp(t *testing.T) {
	t.Parallel()

	var cli struct {
		Version  string `help:"Release version." completion-predictor:"apifield:postgres_version"`
		Location string `help:"Where it is created" completion-predictor:"apifield:postgres_location"`
		Bare     string `completion-predictor:"apifield:postgres_version"`
		NoValues string `help:"Available amount of memory." completion-predictor:"apifield:keyvaluestore_memory_size"`
		Name     string `help:"Name of an existing instance." completion-predictor:"client:resource_name"`
		Interp   string `help:"Defaults to ${a_default}." completion-predictor:"apifield:postgres_version"`
		Plain    string `help:"No predictor at all."`
	}

	parser := kong.Must(&cli,
		kong.Vars{"a_default": "17"},
		kong.PostBuild(Apply()),
	)

	help := map[string]string{}
	for _, flag := range parser.Model.Flags {
		help[flag.Name] = flag.Help
	}

	versions := "Available values: " + strings.Join(fields["postgres_version"].Values, ", ") + "."
	locations := "Available values: " + strings.Join(fields["postgres_location"].Values, ", ") + "."

	is := require.New(t)
	is.Equal("Release version. "+versions, help["version"], "sentence appended after a full stop")
	is.Equal("Where it is created. "+locations, help["location"], "full stop inserted when help has none")
	is.Equal(versions, help["bare"], "no leading separator when there is no help")
	is.Equal("Available amount of memory.", help["no-values"], "a field which only carries a default documents no values")
	is.Equal("Name of an existing instance.", help["name"], "a predictor which is not a field is left alone")
	is.Equal("Defaults to 17. "+versions, help["interp"], "the hook runs before kong interpolates")
	is.Equal("No predictor at all.", help["plain"])
}

func TestApplyPlaceholder(t *testing.T) {
	t.Parallel()

	var cli struct {
		Version   string `completion-predictor:"apifield:postgres_version"`
		Own       string `placeholder:"17" completion-predictor:"apifield:postgres_version"`
		NoValues  string `completion-predictor:"apifield:keyvaluestore_memory_size"`
		NoDefault string `completion-predictor:"apifield:cloudvm_os"`
		Name      string `completion-predictor:"client:resource_name"`
	}

	parser := kong.Must(&cli, kong.PostBuild(Apply()))

	placeholder := map[string]string{}
	for _, flag := range parser.Model.Flags {
		placeholder[flag.Name] = flag.PlaceHolder
	}

	is := require.New(t)
	is.Equal(fields["postgres_version"].Default, placeholder["version"], "the default of the field")
	is.Equal("17", placeholder["own"], "a placeholder of its own is kept")
	is.Equal(fields["keyvaluestore_memory_size"].Default, placeholder["no-values"], "a field which only carries a default")
	is.Empty(placeholder["no-default"], "a field without a default leaves the placeholder alone")
	is.Empty(placeholder["name"], "a predictor which is not a field is left alone")
}

func TestApplyUnknownField(t *testing.T) {
	t.Parallel()

	// Keep in sync with the tag below, which cannot interpolate.
	const unknown = "postgres_no_such_field"

	var cli struct {
		Unknown string `completion-predictor:"apifield:postgres_no_such_field"`
	}

	require.NotContains(t, fields, unknown, "the test needs a field which does not exist")

	_, err := kong.New(&cli, kong.PostBuild(Apply()))

	require.ErrorContains(t, err, unknown)
}

func TestAppendSentence(t *testing.T) {
	t.Parallel()

	for help, expected := range map[string]string{
		"":              "Values.",
		"Ends in stop.": "Ends in stop. Values.",
		"No stop":       "No stop. Values.",
		"Trailing  ":    "Trailing. Values.",
		"Really?":       "Really? Values.",
	} {
		require.Equal(t, expected, appendSentence(help, "Values."), "help: %q", help)
	}
}

func TestPredictorsAreACopy(t *testing.T) {
	t.Parallel()

	Predictors()[qualify("postgres_version")].Predict(complete.Args{})[0] = "tampered"

	require.NotEqual(t, "tampered", fields["postgres_version"].Values[0])
}

func TestPredictorsAreComplete(t *testing.T) {
	t.Parallel()

	predictors := Predictors()

	is := require.New(t)
	for name, f := range fields {
		predictor, ok := predictors[qualify(name)]
		is.True(ok, "field %q has no predictor", name)
		is.NotNil(predictor, "field %q predicts nil", name)
		is.ElementsMatch(f.Values, predictor.Predict(complete.Args{}), "field: %q", name)
	}
}
