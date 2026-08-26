// Package main is the entry point for nctl.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/alecthomas/kong"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/apply"
	"github.com/ninech/nctl/auth"
	"github.com/ninech/nctl/copy"
	"github.com/ninech/nctl/create"
	"github.com/ninech/nctl/delete"
	"github.com/ninech/nctl/edit"
	"github.com/ninech/nctl/exec"
	"github.com/ninech/nctl/get"
	"github.com/ninech/nctl/internal/apifield"
	"github.com/ninech/nctl/internal/cli"
	"github.com/ninech/nctl/internal/completion"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/logs"
	"github.com/ninech/nctl/update"
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

const (
	defaultAPICluster = "nineapis.ch"
)

var (
	version string
	commit  string
	date    string

	writer = os.Stdout
	reader = os.Stdin
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setupSignalHandler(ctx, cancel)

	cmd := &rootCommand{}
	parser, err := newParser(ctx, cmd, writer, reader)
	if err != nil {
		fmt.Fprintln(writer, err)
		os.Exit(1)
	}

	kongCtx, err := parser.Parse(os.Args[1:])
	if err != nil {
		if parseErr, ok := errors.AsType[*kong.ParseError](err); ok {
			// do not error on missing command/argument.
			// Print Usage + friendly message instead.
			if parseErr.Context.Error == nil {
				node := parseErr.Context.Selected()
				if node == nil {
					node = parseErr.Context.Model.Node
				}
				if format.MissingChildren(node) {
					err = format.ExitIfErrorf(writer, err, parseErr.Context.Command())
				}
			}
		}

		parser.FatalIfErrorf(err)
	}

	binds := []any{
		ctx,
		kong.BindTo(writer, (*io.Writer)(nil)),
		kong.BindTo(reader, (*io.Reader)(nil)),
	}
	// Kong exits during Parse for --help and --version, so those cases
	// never reach here. Only auth and completions commands remain.
	if !noAPIClientRequired(kongCtx.Command()) {
		client, err := api.New(
			ctx,
			cmd.APICluster,
			cmd.Project,
			api.LogClient(ctx, cmd.LogAPIAddress, cmd.LogAPIInsecure),
			api.DefaultAnnotations(cli.ManagedByAnnotation, cli.Name),
		)
		if err != nil {
			fmt.Fprintln(writer, err)
			fmt.Fprintf(writer, "\nUnable to get API client, are you logged in?\n\nUse `%s` to login.\n", format.Command().Login())
			os.Exit(1)
		}
		binds = append(binds, client)
	}

	if err := kongCtx.Run(binds...); err != nil {
		if k8serrors.IsForbidden(err) {
			if client := findClient(binds); client != nil {
				org, _ := client.Organization()
				err = cli.ErrorWithContext(err).
					WithExitCode(cli.ExitForbidden).
					WithContext("Organization", org).
					WithContext("Project", client.Project).
					WithSuggestions(
						"Verify in Cockpit Access Management that you are a member of the organization:\nhttps://cockpit.nine.ch/en/customer/contacts\n",
						fmt.Sprintf("List available projects: %s", format.Command().GetProjects()),
						fmt.Sprintf("Check your current session: %s", format.Command().WhoAmI()),
					)
			} else {
				err = cli.ErrorWithContext(fmt.Errorf("permission denied: verify in Cockpit Access Management that you are a member of the organization")).
					WithExitCode(cli.ExitForbidden)
			}
		}

		if cliErr, ok := errors.AsType[*cli.Error](err); ok {
			fmt.Fprintln(writer, err.Error())
			kongCtx.Exit(cliErr.ExitCode())
			return
		}

		kongCtx.FatalIfErrorf(err)
	}
}

// newParser builds the Kong parser for cmd. The given writer and reader are
// bound as [io.Writer] and [io.Reader] so that commands can print output and
// prompt for input.
func newParser(ctx context.Context, cmd *rootCommand, w io.Writer, r io.Reader) (*kong.Kong, error) {
	kongVars, err := kongVariables()
	if err != nil {
		return nil, err
	}

	parser, err := kong.New(
		cmd,
		kong.Name(cli.Name),
		kong.Description(
			"Interact with Nine API resources. See https://docs.nineapis.ch for the full API docs.",
		),
		kong.Groups{
			"verbs": "Resource Management Commands",
			"utils": "Utility Commands",

			"get-general": "General",
			"get-access":  "Project & Access",
			"get-infra":   "Infrastructure",
			"get-apps":    "Applications",
			"get-storage": "Databases & Object Storage",
			"get-network": "Networking",

			"create-general": "General",
			"create-access":  "Project & Access",
			"create-infra":   "Infrastructure",
			"create-apps":    "Applications",
			"create-storage": "Databases & Object Storage",
			"create-network": "Networking",

			"update-access":  "Project & Access",
			"update-infra":   "Infrastructure",
			"update-apps":    "Applications",
			"update-storage": "Databases & Object Storage",
			"update-network": "Networking",

			"edit-access":  "Project & Access",
			"edit-infra":   "Infrastructure",
			"edit-apps":    "Applications",
			"edit-storage": "Databases & Object Storage",

			"delete-general": "General",
			"delete-access":  "Project & Access",
			"delete-infra":   "Infrastructure",
			"delete-apps":    "Applications",
			"delete-storage": "Databases & Object Storage",
			"delete-network": "Networking",
		},
		kong.ConfigureHelp(kong.HelpOptions{
			Compact:             true,
			NoExpandSubcommands: true,
		}),
		kong.UsageOnError(),
		kong.PostBuild(format.InterpolateFlagPlaceholders(kongVars)),
		kong.PostBuild(apifield.Apply()),
		kongVars,
		kong.BindTo(ctx, (*context.Context)(nil)),
		kong.BindTo(w, (*io.Writer)(nil)),
		kong.BindTo(r, (*io.Reader)(nil)),
	)
	if err != nil {
		return nil, err
	}

	// Completion has to be registered before parsing, so cmd is still empty
	// here. The predictors resolve the flags they need off the parser model
	// and the command line being completed instead.
	if err := completion.Register(ctx, parser); err != nil {
		return nil, err
	}

	return parser, nil
}

// noAPIClientRequired returns true if the command does not need to (or can't)
// require an API client. The command parameter is the resolved command path
// from [kong.Context.Command].
func noAPIClientRequired(command string) bool {
	return matchCommand(command, auth.CmdName, format.LoginCommand) ||
		matchCommand(command, auth.CmdName, format.LogoutCommand) ||
		matchCommand(command, auth.CmdName, auth.OIDCCmdName) ||
		matchCommand(command, auth.CmdName, auth.ClientCredentialsCmdName) ||
		matchCommand(command, "completions")
}

func matchCommand(command string, parts ...string) bool {
	return strings.HasPrefix(command, strings.Join(parts, " "))
}

func setupSignalHandler(ctx context.Context, cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		defer func() {
			signal.Stop(c)
		}()

		select {
		case <-c:
			cancel()
		case <-ctx.Done():
		}
	}()
}

// kongVariables collects all variables which should be passed to kong. It
// checks for variables which would overwrite already existing ones.
func kongVariables() (kong.Vars, error) {
	result := make(kong.Vars)
	result["version"] = versionOutput(version, commit, date)
	result["api_cluster"] = defaultAPICluster
	appCreateKongVars, err := create.ApplicationKongVars()
	if err != nil {
		return nil, fmt.Errorf("error on application create kong vars: %w", err)
	}
	if err := merge(
		result,
		appCreateKongVars,
		create.MySQLKongVars(),
		create.ServiceConnectionKongVars(),
		create.BucketKongVars(),
		update.BucketKongVars(),
		auth.LoginKongVars(),
		logs.KongVars(),
	); err != nil {
		return nil, fmt.Errorf("error when merging kong variables: %w", err)
	}

	return result, nil
}

func versionOutput(version, commit, date string) string {
	var extra []string

	if commit != "" {
		extra = append(extra, "commit: "+commit)
	}
	if date != "" {
		extra = append(extra, "date: "+date)
	}
	if len(extra) > 0 {
		return fmt.Sprintf("%s (%s)", version, strings.Join(extra, ", "))
	}
	return version
}

func merge(existing kong.Vars, additional ...kong.Vars) error {
	for _, v := range additional {
		for k, v := range v {
			_, exists := existing[k]
			if exists {
				return fmt.Errorf("variable %q is already in use", k)
			}
			existing[k] = v
		}
	}

	return nil
}

// findClient searches for an *api.Client in the provided binds slice
func findClient(binds []any) *api.Client {
	for _, b := range binds {
		if client, ok := b.(*api.Client); ok {
			return client
		}
	}
	return nil
}

func init() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	if version == "" {
		version = info.Main.Version
	}

	for _, kv := range info.Settings {
		switch kv.Key {
		case "vcs.revision":
			commit = kv.Value
		case "vcs.time":
			date = kv.Value
		}
	}
}
