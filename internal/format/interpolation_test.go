package format

import (
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/require"
)

func TestPlaceholderInterpolation(t *testing.T) {
	t.Parallel()

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
	is := require.New(t)
	is.NoError(err)
	// we expect 3 flags because the first one is always the "-h" help flag
	is.Len(p.Model.Flags, 3)
	flagPointer := p.Model.Flags[1]
	is.Equal("chicken", flagPointer.PlaceHolder)
	// we expect one sub command
	is.Len(p.Model.Children, 1)
	// no help flag this time...
	is.Len(p.Model.Children[0].Flags, 2)
	subFlagPointer := p.Model.Children[0].Flags[1]
	is.Equal("coleslaw!", subFlagPointer.PlaceHolder)
}

// TestPlaceholderInterpolationError makes sure that an error gets thrown if a
// variable in a placeholder was not defined
func TestPlaceholderInterpolationError(t *testing.T) {
	t.Parallel()

	var cli struct {
		FlagPointer *string `placeholder:"${flag_default}"`
	}
	_, err := kong.New(
		&cli,
		kong.PostBuild(InterpolateFlagPlaceholders(kong.Vars{"unused": "garbage"})),
	)
	is := require.New(t)
	is.Error(err)
}
