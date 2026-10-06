package completion

import (
	"sort"
	"testing"

	"github.com/ninech/nctl/internal/testutil"
	"github.com/posener/complete"
)

func TestResourcePredictNames(t *testing.T) {
	t.Parallel()

	const project = testutil.DefaultProject
	client := testutil.SetupClient(t,
		testutil.WithObjects(
			testutil.Postgres("one", project, "nine-es34"),
			testutil.Postgres("two", project, "nine-es34"),
		),
		testutil.WithDefaultProject(project),
	)

	got := newResourceName(staticClient(client), testProjectFinder(t), "postgres").
		Predict(complete.Args{})
	sort.Strings(got)

	want := []string{"one", "two"}
	if len(got) != len(want) {
		t.Fatalf("Predict() = %q, want %q", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("Predict()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestResourcePredictNamesOfUnknownResource(t *testing.T) {
	t.Parallel()

	client := testutil.SetupClient(t, testutil.WithDefaultProject(testutil.DefaultProject))

	if got := newResourceName(staticClient(client), testProjectFinder(t), "nosuchthing").
		Predict(complete.Args{}); got != nil {
		t.Errorf("Predict() = %q, want no completions", got)
	}
}
