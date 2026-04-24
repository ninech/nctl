package predictor

import (
	"bytes"
	"sort"
	"strconv"
	"testing"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/internal/test"
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
	t.Parallel()

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
			t.Parallel()

			if got := findProjectInSlice(tt.args); got != tt.want {
				t.Errorf("findProjectInSlice() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFirstPositionalArg(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "empty",
			args: []string{},
			want: "",
		},
		{
			name: "positional only",
			args: []string{"myinstance"},
			want: "myinstance",
		},
		{
			name: "flag before positional",
			args: []string{"-p", "myproject", "myinstance"},
			want: "myinstance",
		},
		{
			name: "positional then flag",
			args: []string{"myinstance", "--database"},
			want: "myinstance",
		},
		{
			name: "flag equals form skips no value token",
			args: []string{"--project=myproject", "myinstance"},
			want: "myinstance",
		},
		{
			name: "only flags",
			args: []string{"-p", "myproject"},
			want: "",
		},
		{
			name: "dangling flag without value",
			args: []string{"--database"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := firstPositionalArg(tt.args); got != tt.want {
				t.Errorf("firstPositionalArg(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestInstanceDatabasesPredict(t *testing.T) {
	t.Parallel()

	const (
		instanceName = "mypg"
		project      = test.DefaultProject
		location     = "nine-es34"
	)

	pg := test.Postgres(instanceName, project, location)
	pg.Status.AtProvider.Databases = map[string]storage.DatabaseObservation{
		"appdb":    {},
		"otherdb":  {},
		"postgres": {},
	}

	client := test.SetupClient(t,
		test.WithObjects(pg),
		test.WithDefaultProject(project),
	)

	predictor := NewInstanceDatabases(client, storage.PostgresGroupVersionKind)

	tests := []struct {
		name      string
		completed []string
		want      []string
	}{
		{
			name:      "returns databases for named instance",
			completed: []string{instanceName, "--database"},
			want:      []string{"appdb", "otherdb", "postgres"},
		},
		{
			name:      "returns nil when no instance name provided",
			completed: []string{"--database"},
			want:      nil,
		},
		{
			name:      "returns nil for unknown instance",
			completed: []string{"doesnotexist", "--database"},
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := predictor.Predict(complete.Args{Completed: tt.completed})
			sort.Strings(got)
			sort.Strings(tt.want)

			if len(got) != len(tt.want) {
				t.Fatalf("Predict() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("Predict()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
