package testutil

import (
	"github.com/alecthomas/kong"
	"github.com/ninech/nctl/internal/format"
)

// KongStreams returns a Kong option binding streams for the hooks of the commands
// and making them the writers of Kong,
// as the parser of nctl does.
func KongStreams(streams format.IOStreams) kong.Option {
	return kong.OptionFunc(func(k *kong.Kong) error {
		if err := kong.Writers(streams.Out, streams.ErrOut).Apply(k); err != nil {
			return err
		}

		return kong.Bind(streams).Apply(k)
	})
}
