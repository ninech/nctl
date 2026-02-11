package main

import (
	"testing"

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
