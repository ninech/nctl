package cli

import (
	"fmt"
	"strings"
)

// Exit codes following the square/exit convention.
// See https://github.com/square/exit for details.
const (
	ExitOK            = 0   // successful execution
	ExitError         = 1   // generic error
	ExitUsageError    = 80  // invalid input, validation errors
	ExitForbidden     = 83  // permission denied
	ExitInternalError = 100 // bug in nctl
	ExitUnavailable   = 101 // API/service unreachable
)

// Error wraps an error with additional context for user-friendly display.
// It implements [kong.ExitCoder] to provide semantic exit codes.
type Error struct {
	// Err is the underlying error being wrapped.
	Err error
	// Code is the exit code for this error. When zero, ExitCode defaults to ExitError (1).
	Code int
	// Context provides additional key-value metadata about the error (e.g., "Organization": "foo").
	Context map[string]string
	// Available lists valid options that could have been used.
	Available []string
	// Suggestions provides recommended commands or actions to fix the error.
	Suggestions []string
}

// Error returns the full formatted error for display to the user,
// including context, available options, and suggestions.
func (e *Error) Error() string {
	sb := strings.Builder{}
	if e.Err != nil {
		sb.WriteString(e.Err.Error())
	}

	if len(e.Context) > 0 {
		sb.WriteString("\n")
		for k, v := range e.Context {
			fmt.Fprintf(&sb, "\n%s: %s", k, v)
		}
	}

	if len(e.Available) > 0 {
		sb.WriteString("\n\nAvailable:")
		for _, item := range e.Available {
			fmt.Fprintf(&sb, "\n  - %s", item)
		}
	}

	if len(e.Suggestions) > 0 {
		sb.WriteString("\n\nSuggested actions:")
		for _, s := range e.Suggestions {
			s = strings.ReplaceAll(s, "\n", "\n    ")
			fmt.Fprintf(&sb, "\n  - %s", s)
		}
	}

	return sb.String()
}

// ExitCode returns the exit code for this error, satisfying [kong.ExitCoder].
// When Code is zero, it defaults to ExitError (1).
func (e *Error) ExitCode() int {
	return e.Code
}

// Unwrap returns the wrapped error for [errors.Is] / [errors.As] support.
func (e *Error) Unwrap() error {
	return e.Err
}

// ErrorWithContext wraps an existing error with contextual information.
// Returns nil if err is nil.
func ErrorWithContext(err error) *Error {
	if err == nil {
		return nil
	}
	return &Error{Err: err}
}

// WithExitCode sets the exit code for the error.
func (e *Error) WithExitCode(code int) *Error {
	e.Code = code
	return e
}

// WithAvailable adds available options to the error.
func (e *Error) WithAvailable(items ...string) *Error {
	e.Available = append(e.Available, items...)
	return e
}

// WithSuggestions adds suggested actions to the error.
func (e *Error) WithSuggestions(suggestions ...string) *Error {
	e.Suggestions = append(e.Suggestions, suggestions...)
	return e
}

// WithContext adds a key-value pair to the error context.
func (e *Error) WithContext(key, value string) *Error {
	if e.Context == nil {
		e.Context = make(map[string]string)
	}
	e.Context[key] = value
	return e
}
