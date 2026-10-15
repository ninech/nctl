// Package cmd is the root of the nctl command tree.
// The verbs live in its subpackages.
package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/alecthomas/kong"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/cli"
	"github.com/ninech/nctl/internal/cmd/apply"
	"github.com/ninech/nctl/internal/cmd/auth"
	"github.com/ninech/nctl/internal/cmd/copy"
	"github.com/ninech/nctl/internal/cmd/create"
	"github.com/ninech/nctl/internal/cmd/delete"
	"github.com/ninech/nctl/internal/cmd/edit"
	"github.com/ninech/nctl/internal/cmd/exec"
	"github.com/ninech/nctl/internal/cmd/get"
	"github.com/ninech/nctl/internal/cmd/logs"
	"github.com/ninech/nctl/internal/cmd/update"
	"github.com/ninech/nctl/internal/completion"
	"github.com/ninech/nctl/internal/format"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
)

type flags struct {
	Project        string           `help:"Limit commands to a specific project." short:"p" completion-predictor:"client:project_name"`
	APICluster     string           `help:"Context name of the API cluster." default:"${api_cluster}" env:"NCTL_API_CLUSTER" hidden:""`
	LogAPIAddress  string           `help:"Address of the deplo.io logging API server." default:"https://logs.deplo.io" env:"NCTL_LOG_ADDR" hidden:""`
	LogAPIInsecure bool             `help:"Don't verify TLS connection to the logging API server." hidden:"" default:"false" env:"NCTL_LOG_INSECURE"`
	Verbose        bool             `help:"Show verbose messages."`
	Version        kong.VersionFlag `name:"version" help:"Print version information and quit."`
}

type rootCommand struct {
	flags

	// Resource management
	Get    get.Cmd    `cmd:"" help:"List resources across Nine APIs and watch them for changes." group:"verbs"`
	Create create.Cmd `cmd:"" help:"Create resources from YAML or JSON files, or from resource-specific subcommands." group:"verbs"`
	Apply  apply.Cmd  `cmd:"" help:"Apply resources declaratively from YAML or JSON files." group:"verbs"`
	Update update.Cmd `cmd:"" help:"Update existing resources using resource-specific subcommands." group:"verbs"`
	Delete delete.Cmd `cmd:"" help:"Delete resources by file or through resource-specific subcommands." group:"verbs"`
	Edit   edit.Cmd   `cmd:"" help:"Edit supported resources interactively in your configured editor." group:"verbs"`

	// Utility & interaction
	Auth        auth.Cmd       `cmd:"" help:"Log in, switch organization or project context, and inspect your current session." group:"utils"`
	Logs        logs.Cmd       `cmd:"" help:"Show logs for supported deplo.io resources such as applications and builds." group:"utils"`
	Exec        exec.Cmd       `cmd:"" help:"Run a command or open a shell in a deplo.io application." group:"utils"`
	Copy        copy.Cmd       `cmd:"" help:"Copy supported resources such as deplo.io applications." group:"utils"`
	Completions completion.Cmd `cmd:"" help:"Generate shell completion commands for your current shell." group:"utils"`
}

// exit is the panic value with which kong's exit hook unwinds [Run].
type exit int

// Run runs nctl with args, which exclude the program name,
// and returns the exit code.
// version is shown by --version.
func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, version string) (code int) {
	// Kong exits on --help, --version, shell completion and its Fatal helpers.
	// The hook turns these exits into a return.
	defer func() {
		if r := recover(); r != nil {
			e, ok := r.(exit)
			if !ok {
				panic(r)
			}
			code = int(e)
		}
	}()

	root := &rootCommand{}
	streams := format.IOStreams{In: stdin, Out: stdout, ErrOut: stderr}
	parser, err := newParser(ctx, root, version, streams,
		kong.Exit(func(code int) { panic(exit(code)) }),
	)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return cli.ExitError
	}
	// fatal reports err and returns the exit code through the exit hook.
	fatal := func(err error) int {
		parser.FatalIfErrorf(err)
		return cli.ExitError
	}

	kongCtx, err := parser.Parse(args)
	if err != nil {
		// Instead of an error on a missing command or argument,
		// print the usage and a friendly message.
		if parseErr, ok := errors.AsType[*kong.ParseError](err); ok && missingCommand(parseErr) {
			if err = printMissingCommand(stderr, parseErr); err == nil {
				return cli.ExitError
			}
		}

		return fatal(err)
	}

	// Kong calls the provider only for commands taking an *api.Client,
	// so commands like auth login work without being logged in.
	var client *api.Client
	if err := kongCtx.BindSingletonProvider(func(ctx context.Context) (*api.Client, error) {
		c, err := newAPIClient(ctx, root.flags)
		client = c
		return c, err
	}); err != nil {
		return fatal(err)
	}

	if err := kongCtx.Run(); err != nil {
		if k8serrors.IsForbidden(err) {
			err = forbiddenError(err, client)
		}

		if cliErr, ok := errors.AsType[*cli.Error](err); ok {
			fmt.Fprintln(stderr, err.Error())
			return cliErr.ExitCode()
		}

		return fatal(err)
	}

	return cli.ExitOK
}

// newAPIClient returns a client for the API cluster and project selected by the flags,
// explaining how to log in when that fails.
func newAPIClient(ctx context.Context, f flags) (*api.Client, error) {
	client, err := api.New(
		ctx,
		f.APICluster,
		f.Project,
		api.LogClient(ctx, f.LogAPIAddress, f.LogAPIInsecure),
		api.DefaultAnnotations(api.ManagedByAnnotation, api.Name),
	)
	if err == nil {
		return client, nil
	}
	if _, ok := errors.AsType[*cli.Error](err); ok {
		return nil, err
	}

	return nil, cli.ErrorWithContext(fmt.Errorf("unable to get API client: %w", err)).
		WithExitCode(cli.ExitError).
		WithSuggestions("Login to the API: " + cli.LoginCmd)
}

// forbiddenError explains how to get access after the API denied a request with err.
// client is nil when the command did not use one.
func forbiddenError(err error, client *api.Client) *cli.Error {
	if client == nil {
		return cli.ErrorWithContext(fmt.Errorf("permission denied: verify in Cockpit Access Management that you are a member of the organization")).
			WithExitCode(cli.ExitForbidden)
	}

	org, _ := client.Organization()
	return cli.ErrorWithContext(err).
		WithExitCode(cli.ExitForbidden).
		WithContext("Organization", org).
		WithContext("Project", client.Project).
		WithSuggestions(
			"Verify in Cockpit Access Management that you are a member of the organization:\nhttps://cockpit.nine.ch/en/customer/contacts\n",
			fmt.Sprintf("List available projects: %s", cli.GetProjectsCmd),
			fmt.Sprintf("Check your current session: %s", cli.WhoAmICmd),
		)
}
