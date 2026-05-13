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

	tests := []struct {
		command  string
		expected bool
	}{
		{"--help", true},
		{"-h", true},
		{"--version", true},
		{"get --help", true},
		{"get application --help", true},
		{"get -h", true},
		{"auth login", true},
		{"auth logout", true},
		{"completions", true},
		{"get", false},
		{"get application", false},
		{"create application", false},
		{"", false},
	}
	for _, tt := range tests {
		is.Equal(tt.expected, noAPIClientRequired(tt.command), "command: %q", tt.command)
	}
}
