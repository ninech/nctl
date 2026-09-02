package completion

import (
	"sort"
	"testing"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/internal/test"
	"github.com/posener/complete"
)

func TestFirstPositionalArg(t *testing.T) {
	t.Parallel()

	// The flags of a command which consume the token following them, as
	// [bindArgFlags] binds them to the predictor.
	argFlags := []string{"--project", "-p", "--database", "-d"}

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
		{
			name: "boolean flag before positional",
			args: []string{"--verbose", "myinstance"},
			want: "myinstance",
		},
		{
			name: "boolean flag between value flag and positional",
			args: []string{"-p", "myproject", "--verbose", "myinstance"},
			want: "myinstance",
		},
		{
			name: "boolean flags only",
			args: []string{"--verbose", "--all-projects"},
			want: "",
		},
		{
			name: "positional after end of flags",
			args: []string{"--verbose", "--", "myinstance"},
			want: "myinstance",
		},
		{
			name: "dangling end of flags",
			args: []string{"--"},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := firstPositionalArg(tt.args, argFlags); got != tt.want {
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

	predictor := newInstanceDatabases(staticClient(client), testProjectFinder(t), storage.PostgresGroupVersionKind)

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
