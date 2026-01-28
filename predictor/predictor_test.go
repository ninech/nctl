package predictor

import "testing"

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
