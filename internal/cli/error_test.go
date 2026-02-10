package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
)

// Ensure that Error implements the [kong.ExitCoder] interface.
var _ kong.ExitCoder = &Error{}

func TestErrorExitCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *Error
		wantCode int
	}{
		{
			name:     "default exit code when Code is zero",
			err:      &Error{Err: errors.New("fail")},
			wantCode: 0,
		},
		{
			name:     "explicit usage error",
			err:      &Error{Err: errors.New("bad input"), Code: ExitUsageError},
			wantCode: ExitUsageError,
		},
		{
			name:     "explicit forbidden",
			err:      &Error{Err: errors.New("denied"), Code: ExitForbidden},
			wantCode: ExitForbidden,
		},
		{
			name:     "WithExitCode builder",
			err:      ErrorWithContext(errors.New("test")).WithExitCode(ExitUnavailable),
			wantCode: ExitUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.err.ExitCode(); got != tt.wantCode {
				t.Errorf("ExitCode() = %d, want %d", got, tt.wantCode)
			}
		})
	}
}

func TestErrorOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		err      *Error
		contains []string
	}{
		{
			name:     "nil underlying error",
			err:      &Error{},
			contains: []string{},
		},
		{
			name:     "message only",
			err:      ErrorWithContext(errors.New("something broke")),
			contains: []string{"something broke"},
		},
		{
			name: "with context",
			err: ErrorWithContext(errors.New("denied")).
				WithContext("Organization", "acme"),
			contains: []string{"denied", `Organization: acme`},
		},
		{
			name: "with available options",
			err: ErrorWithContext(errors.New("invalid")).
				WithAvailable("foo", "bar"),
			contains: []string{"Available:", "- foo", "- bar"},
		},
		{
			name: "with suggestions",
			err: ErrorWithContext(errors.New("not found")).
				WithSuggestions("try this", "or that"),
			contains: []string{"Suggested actions:", "- try this", "- or that"},
		},
		{
			name: "full error",
			err: ErrorWithContext(errors.New("resource not found")).
				WithContext("Project", "myproj").
				WithAvailable("proj-a", "proj-b").
				WithSuggestions("use --project"),
			contains: []string{
				"resource not found",
				`Project: myproj`,
				"Available:",
				"- proj-a",
				"- proj-b",
				"Suggested actions:",
				"- use --project",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.err.Error()
			for _, want := range tt.contains {
				if !strings.Contains(strings.ToLower(got), strings.ToLower(want)) {
					t.Errorf("Error() output missing %q\ngot: %s", want, got)
				}
			}
		})
	}
}

func TestErrorUnwrap(t *testing.T) {
	t.Parallel()

	underlying := errors.New("root cause")
	err := ErrorWithContext(underlying)

	if !errors.Is(err, underlying) {
		t.Error("errors.Is should match the underlying error")
	}

	var cliErr *Error
	if !errors.As(err, &cliErr) {
		t.Error("errors.As should find *cli.Error")
	}
}

func TestErrorWithContextNil(t *testing.T) {
	t.Parallel()

	if got := ErrorWithContext(nil); got != nil {
		t.Errorf("ErrorWithContext(nil) = %v, want nil", got)
	}
}
