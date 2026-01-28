package predictor

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/posener/complete"
)

func TestFindProjectWithCompleteLibrary(t *testing.T) {
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

			// Simulate shell completion
			t.Setenv("COMP_LINE", tt.compLine)
			t.Setenv("COMP_POINT", strconv.Itoa(len(tt.compLine)))

			cmp := complete.New("nctl", cmd)
			cmp.Out = &bytes.Buffer{} // discard output
			cmp.Complete()

			if !capture.called {
				t.Fatal("predictor was not called")
			}

			gotProject, _ := findProject(capture.captured)
			if gotProject != tt.wantProject {
				t.Errorf("findProject() = %q, want %q", gotProject, tt.wantProject)
			}
		})
	}
}

func TestFindProjectIncomplete(t *testing.T) {
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

			// For incomplete flags, the completion happens at the exec level
			// (completing the project name), not at the positional arg level.
			cmd := complete.Command{
				Sub: map[string]complete.Command{
					"exec": {
						Flags: map[string]complete.Predictor{
							"--project": capture, // capture here for incomplete flag tests
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

			_, gotIncomplete := findProject(capture.captured)
			if gotIncomplete != tt.wantIncomplete {
				t.Errorf("findProject() incomplete = %v, want %v (LastCompleted=%q)",
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

func TestFindProjectInSlice(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "empty args",
			args: []string{},
			want: "",
		},
		{
			name: "no project flag",
			args: []string{"nctl", "get", "applications"},
			want: "",
		},
		{
			name: "short flag with value",
			args: []string{"nctl", "-p", "myproject", "get", "applications"},
			want: "myproject",
		},
		{
			name: "long flag with value",
			args: []string{"nctl", "--project", "myproject", "get", "applications"},
			want: "myproject",
		},
		{
			name: "flag at end with value",
			args: []string{"nctl", "get", "applications", "-p", "myproject"},
			want: "myproject",
		},
		{
			name: "short flag without value (incomplete)",
			args: []string{"nctl", "get", "applications", "-p"},
			want: "",
		},
		{
			name: "long flag without value (incomplete)",
			args: []string{"nctl", "get", "applications", "--project"},
			want: "",
		},
		{
			name: "flag in middle of args",
			args: []string{"nctl", "get", "-p", "proj", "applications"},
			want: "proj",
		},
		{
			name: "multiple flags takes first",
			args: []string{"nctl", "-p", "first", "get", "-p", "second"},
			want: "first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findProjectInSlice(tt.args); got != tt.want {
				t.Errorf("findProjectInSlice() = %q, want %q", got, tt.want)
			}
		})
	}
}
