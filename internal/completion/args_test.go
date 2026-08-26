package completion

import (
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"
)

var testFlags = []string{"-p", "--project"}

func TestFlagValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		args  []string
		names []string
		want  string
	}{
		{
			name:  "empty args",
			args:  []string{},
			names: testFlags,
			want:  "",
		},
		{
			name:  "no project flag",
			args:  []string{"nctl", "get", "applications"},
			names: testFlags,
			want:  "",
		},
		{
			name:  "short flag with value",
			args:  []string{"nctl", "-p", "myproject", "get", "applications"},
			names: testFlags,
			want:  "myproject",
		},
		{
			name:  "long flag with value",
			args:  []string{"nctl", "--project", "myproject", "get", "applications"},
			names: testFlags,
			want:  "myproject",
		},
		{
			name:  "long flag with assigned value",
			args:  []string{"nctl", "--project=myproject", "get", "applications"},
			names: testFlags,
			want:  "myproject",
		},
		{
			name:  "short flag with assigned value",
			args:  []string{"nctl", "-p=myproject", "get", "applications"},
			names: testFlags,
			want:  "myproject",
		},
		{
			name:  "assigned empty value",
			args:  []string{"nctl", "--project=", "get", "applications"},
			names: testFlags,
			want:  "",
		},
		{
			name:  "flag at end with value",
			args:  []string{"nctl", "get", "applications", "-p", "myproject"},
			names: testFlags,
			want:  "myproject",
		},
		{
			name:  "short flag without value (incomplete)",
			args:  []string{"nctl", "get", "applications", "-p"},
			names: testFlags,
			want:  "",
		},
		{
			name:  "long flag without value (incomplete)",
			args:  []string{"nctl", "get", "applications", "--project"},
			names: testFlags,
			want:  "",
		},
		{
			name:  "flag in middle of args",
			args:  []string{"nctl", "get", "-p", "proj", "applications"},
			names: testFlags,
			want:  "proj",
		},
		{
			name:  "multiple flags takes first",
			args:  []string{"nctl", "-p", "first", "get", "-p", "second"},
			names: testFlags,
			want:  "first",
		},
		{
			name:  "flag name is only matched in full",
			args:  []string{"nctl", "--projects", "myproject", "get"},
			names: testFlags,
			want:  "",
		},
		{
			name:  "no names to match",
			args:  []string{"nctl", "-p", "myproject"},
			names: nil,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tt.want, flagValue(tt.args, tt.names...))
		})
	}
}

func TestFlagNames(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	var grammar struct {
		Project string `short:"p" aliases:"namespace"`
		Verbose bool
	}

	parser, err := kong.New(&grammar)
	is.NoError(err)

	is.Equal([]string{"--project", "-p", "--namespace"}, flagNames(findFlag(parser, "project")))
	is.Equal([]string{"--verbose"}, flagNames(findFlag(parser, "verbose")))
	is.Nil(flagNames(nil))
}

func TestFindFlag(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	var grammar struct {
		Project string
	}

	parser, err := kong.New(&grammar)
	is.NoError(err)

	is.NotNil(findFlag(parser, "project"))
	is.Nil(findFlag(parser, "unknown"))
	is.Nil(findFlag(nil, "project"))
}
