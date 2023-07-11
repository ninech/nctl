package format

import (
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"
	"gotest.tools/assert"
)

func TestPlaceholderInterpolation(t *testing.T) {
	var cli struct {
		FlagPointer *string `placeholder:"${flag_default}"`
		Flag        string  `default:"${flag_default}"`
		Sub         struct {
			SubFlag        int  `default:"${subflag_default}"`
			SubFlagPointer *int `placeholder:"${subflag_default}"`
		} `cmd:""`
	}
	vars := kong.Vars{
		"flag_default":    "chicken",
		"subflag_default": "coleslaw!",
	}

	p, err := kong.New(
		&cli,
		vars,
		kong.PostBuild(InterpolateFlagPlaceholders(vars)),
	)
	require.NoError(t, err)
	// we expect 3 flags because the first one is always the "-h" help flag
	require.Len(t, p.Model.Flags, 3)
	flagPointer := p.Model.Flags[1]
	assert.Equal(t, "chicken", flagPointer.PlaceHolder)
	// we expect one sub command
	require.Len(t, p.Model.Children, 1)
	// no help flag this time...
	require.Len(t, p.Model.Children[0].Flags, 2)
	subFlagPointer := p.Model.Children[0].Flags[1]
	require.Equal(t, "coleslaw!", subFlagPointer.PlaceHolder)
}

// TestPlaceholderInterpolationError makes sure that an error gets thrown if a
// variable in a placeholder was not defined
func TestPlaceholderInterpolationError(t *testing.T) {
	var cli struct {
		FlagPointer *string `placeholder:"${flag_default}"`
	}
	_, err := kong.New(
		&cli,
		kong.PostBuild(InterpolateFlagPlaceholders(kong.Vars{"unused": "garbage"})),
	)
	require.Error(t, err)
}
