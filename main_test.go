package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestKongVars makes sure that the kongVariables function will not run into an
// error. As it is based mostly on static input, a simple test should be enough.
func TestKongVars(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	vars, err := kongVariables()
	is.NoError(err)
	is.NotEmpty(vars)
}

func TestNoAPIClientRequired(t *testing.T) {
	t.Parallel()
	is := assert.New(t)

	// Commands use the resolved format from kong.Context.Command().
	// --help and --version are not tested here because Kong exits
	// during Parse before noAPIClientRequired is called.
	tests := []struct {
		command  string
		expected bool
	}{
		{"auth login", true},
		{"auth login <organization>", true},
		{"auth logout", true},
		{"auth oidc", true},
		{"auth client-credentials", true},
		{"completions", true},
		{"completions bash", true},
		{"get", false},
		{"get application", false},
		{"get application <name>", false},
		{"create application <name>", false},
		{"exec application <name>", false},
		{"", false},
	}
	for _, tt := range tests {
		is.Equal(tt.expected, noAPIClientRequired(tt.command), "command: %q", tt.command)
	}
}
