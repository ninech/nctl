package test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/alecthomas/kong"
	"github.com/ninech/nctl/api"
)

// RunNamedWithFlags is a generic test helper for running Kong command.
// Parameters:
//   - cli:        pointer to the CLI root (e.g. &struct{ Bucket *bucketCmd `cmd:"" name:"bucket"` }{})
//   - cmdPath:    command path (e.g. []string{"bucket"} or {"bucket","update"})
//   - name:       optional positional NAME; if empty, defaults to "test-"+t.Name()
//   - flags:      command-line flags only (the helper prepends cmdPath + NAME)
//   - afterParse: closure returning pointers to defaulted fields (e.g. Name, WaitTimeout)
//   - clientOpts: passthrough options for test.SetupClient (e.g. test.WithObjects(...))
//
// Safe to use with t.Parallel(): each test gets a unique name.
// Inspired by https://github.com/alecthomas/kong/blob/v1.12.1/kong_test.go#L454
func RunNamedWithFlags(
	t *testing.T,
	cli any,
	vars kong.Vars,
	cmdPath []string,
	name string,
	flags []string,
	afterParse func() (namePtr *string, waitPtr *bool, waitTimeoutPtr *time.Duration),
	clientOpts ...ClientSetupOption,
) (*api.Client, string, error) {
	t.Helper()

	if name == "" {
		name = "test-" + t.Name()
	}
	// Build args: <cmdPath...> <NAME> <flags...>
	args := append(append([]string{}, cmdPath...), name)
	args = append(args, flags...)

	apiClient, err := SetupClient(clientOpts...)
	if err != nil {
		return nil, name, err
	}

	parser := kong.Must(
		cli,
		kong.Name("nctl-test"),
		vars,
		kong.BindTo(t.Context(), (*context.Context)(nil)),
		kong.BindTo(t.Output(), (*io.Writer)(nil)),
	)

	kctx, err := parser.Parse(args)
	if err != nil {
		return nil, name, err
	}

	// Inject per-test defaults after parsing (only if fields exist & are zero).
	if afterParse != nil {
		if namePtr, waitPtr, waitTimeoutPtr := afterParse(); namePtr != nil {
			if *namePtr == "" {
				*namePtr = name
			}
			if waitPtr != nil {
				*waitPtr = false
			}
			if waitTimeoutPtr != nil && *waitTimeoutPtr == 0 {
				*waitTimeoutPtr = time.Second
			}
		}
	}

	if err := kctx.Run(apiClient); err != nil {
		return nil, name, err
	}
	return apiClient, name, nil
}
