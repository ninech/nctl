package completion

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/posener/complete"
	"github.com/stretchr/testify/require"
)

func TestProjectFinderWithCompleteLibrary(t *testing.T) {
	tests := []struct {
		name        string
		compLine    string
		wantProject string
	}{
		{
			name:        "project flag before positional arg",
			compLine:    "nctl exec --project myproject application ",
			wantProject: "myproject",
		},
		{
			name:        "short project flag",
			compLine:    "nctl exec -p myproject application ",
			wantProject: "myproject",
		},
		{
			name:        "no project flag",
			compLine:    "nctl exec application ",
			wantProject: "",
		},
		{
			name:        "project flag after subcommand",
			compLine:    "nctl exec application --project otherproject ",
			wantProject: "otherproject",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capture := &capturePredictor{predictions: []string{"test-result"}}

			// Build a command structure similar to what kong-completion generates
			// for "nctl exec application <name>".
			cmd := complete.Command{
				Sub: map[string]complete.Command{
					"exec": {
						Flags: map[string]complete.Predictor{
							"--project": complete.PredictAnything,
							"-p":        complete.PredictAnything,
						},
						Sub: map[string]complete.Command{
							"application": {
								Args: capture,
							},
						},
					},
				},
			}

			t.Setenv("COMP_LINE", tt.compLine)
			t.Setenv("COMP_POINT", strconv.Itoa(len(tt.compLine)))

			cmp := complete.New("nctl", cmd)
			cmp.Out = &bytes.Buffer{}
			cmp.Complete()

			if !capture.called {
				t.Fatal("predictor was not called")
			}

			gotProject, _ := testProjectFinder(t).find(capture.captured)
			if gotProject != tt.wantProject {
				t.Errorf("find() = %q, want %q", gotProject, tt.wantProject)
			}
		})
	}
}

func TestProjectFinderIncomplete(t *testing.T) {
	tests := []struct {
		name           string
		compLine       string
		wantIncomplete bool
	}{
		{
			name:           "incomplete --project flag",
			compLine:       "nctl exec --project ",
			wantIncomplete: true,
		},
		{
			name:           "incomplete -p flag",
			compLine:       "nctl exec -p ",
			wantIncomplete: true,
		},
		{
			name:           "complete project flag",
			compLine:       "nctl exec --project myproject ",
			wantIncomplete: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capture := &capturePredictor{predictions: []string{}}

			// For incomplete flags, completion happens at the exec level
			// (completing the project name), not at the positional arg level.
			cmd := complete.Command{
				Sub: map[string]complete.Command{
					"exec": {
						Flags: map[string]complete.Predictor{
							"--project": capture,
							"-p":        capture,
						},
						Sub: map[string]complete.Command{
							"application": {
								Args: capture,
							},
						},
					},
				},
			}

			t.Setenv("COMP_LINE", tt.compLine)
			t.Setenv("COMP_POINT", strconv.Itoa(len(tt.compLine)))

			cmp := complete.New("nctl", cmd)
			cmp.Out = &bytes.Buffer{}
			cmp.Complete()

			_, gotIncomplete := testProjectFinder(t).find(capture.captured)
			if gotIncomplete != tt.wantIncomplete {
				t.Errorf("find() incomplete = %v, want %v (LastCompleted=%q)",
					gotIncomplete, tt.wantIncomplete, capture.captured.LastCompleted)
			}
		})
	}
}

// capturePredictor is a test predictor that captures the args it receives.
type capturePredictor struct {
	captured    complete.Args
	predictions []string
	called      bool
}

func (c *capturePredictor) Predict(args complete.Args) []string {
	c.captured = args
	c.called = true
	return c.predictions
}

func TestProjectFinderWithoutFlag(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	var grammar struct {
		Verbose bool
	}

	parser, err := kong.New(&grammar)
	is.NoError(err)

	project, incomplete := newProjectFinder(parser).find(complete.Args{
		All:           []string{"get", "-p", "myproject"},
		LastCompleted: "-p",
	})
	is.Empty(project, "a CLI without a project flag finds no project")
	is.False(incomplete, "a CLI without a project flag has nothing to complete")

	is.Empty(newProjectFinder(nil))
}

// testProjectFinder returns a finder for a project flag declared the same way
// the CLI declares it.
func testProjectFinder(t *testing.T) projectFinder {
	t.Helper()

	var grammar struct {
		Project string `help:"Limit commands to a specific project." short:"p"`
	}

	parser, err := kong.New(&grammar)
	require.NoError(t, err)

	return newProjectFinder(parser)
}
