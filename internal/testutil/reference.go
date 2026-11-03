package testutil

import (
	"testing"

	"github.com/ninech/nctl/internal/application"
	"github.com/stretchr/testify/require"
)

// TypedReference parses ref in the form kind/name like a flag of that type.
func TypedReference(t testing.TB, ref string) application.TypedReference {
	t.Helper()

	var r application.TypedReference
	require.NoError(t, r.UnmarshalText([]byte(ref)))
	return r
}
