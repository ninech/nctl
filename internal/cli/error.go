package cli

import (
	"fmt"
	"strings"

	"github.com/ninech/nctl/internal/format"
)

// Exit codes following the square/exit convention.
// See https://pkg.go.dev/github.com/square/exit#readme-the-codes for details.
const (
	// ExitOK indicates that the program exited successfully.
	ExitOK = 0
	// ExitError indicates that the program exited unsuccessfully
	// but gives no extra context as to what the failure was.
	ExitError = 1
	// ExitUsageError indicates that the program exited unsuccessfully
	// because it was used incorrectly.
	//
	// Examples: a required argument was omitted or an invalid value
	// was supplied for a flag.
	ExitUsageError = 80
	// ExitForbidden indicates that the program exited unsuccessfully
	// because the user isn't authorized to perform the requested action.
	ExitForbidden = 83
	// ExitInternalError indicates that the program exited unsuccessfully
	// because of a problem in its own code.
	//
	// Used instead of 1 when the problem is known to be with the program's
	// code or dependencies.
	ExitInternalError = 100
	// ExitUnavailable indicates that the program exited unsuccessfully
	// because a service it depends on was not available.
	//
	// Examples: A local daemon or remote service did not respond, a connection
	// was closed unexpectedly, an HTTP service responded with 503.
	ExitUnavailable = 101
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
		s := e.Err.Error()
		sb.WriteString(format.Failuref("💥", "%s", strings.ToUpper(s[:1])+s[1:]))
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
