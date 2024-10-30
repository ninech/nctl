package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/alecthomas/kong"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	completion "github.com/jotaen/kong-completion"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/apply"
	"github.com/ninech/nctl/auth"
	"github.com/ninech/nctl/create"
	"github.com/ninech/nctl/delete"
	"github.com/ninech/nctl/exec"
	"github.com/ninech/nctl/get"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/logs"
	"github.com/ninech/nctl/predictor"
	"github.com/ninech/nctl/update"
	"github.com/posener/complete"
)

type flags struct {
	Project        string           `predictor:"resource_name" help:"Limit commands to a specific project." short:"p"`
	APICluster     string           `help:"Context name of the API cluster." default:"${api_cluster}" env:"NCTL_API_CLUSTER"`
	LogAPIAddress  string           `help:"Address of the deplo.io logging API server." default:"https://logs.deplo.io" env:"NCTL_LOG_ADDR"`
	LogAPIInsecure bool             `help:"Don't verify TLS connection to the logging API server." hidden:"" default:"false" env:"NCTL_LOG_INSECURE"`
	Version        kong.VersionFlag `name:"version" help:"Print version information and quit."`
}

type rootCommand struct {
	flags
	Get         get.Cmd               `cmd:"" help:"Get resource."`
	Auth        auth.Cmd              `cmd:"" help:"Authenticate with resource."`
	Completions completion.Completion `cmd:"" help:"Print shell completions."`
	Create      create.Cmd            `cmd:"" help:"Create resource."`
	Apply       apply.Cmd             `cmd:"" help:"Apply resource."`
	Delete      delete.Cmd            `cmd:"" help:"Delete resource."`
	Logs        logs.Cmd              `cmd:"" help:"Get logs of resource."`
	Update      update.Cmd            `cmd:"" help:"Update resource."`
	Exec        exec.Cmd              `cmd:"" help:"Execute a command."`
}

const (
	defaultAPICluster = "nineapis.ch"
)

var (
	version string
	commit  string
	date    string
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	setupSignalHandler(ctx, cancel)

	kongVars, err := kongVariables()
	if err != nil {
		log.Fatal(err)
	}
	nctl := &rootCommand{}
	parser := kong.Must(
		nctl,
		kong.Name(util.NctlName),
		kong.Description("Interact with Nine API resources. See https://docs.nineapis.ch for the full API docs."),
		kong.UsageOnError(),
		kong.PostBuild(format.InterpolateFlagPlaceholders(kongVars)),
		kongVars,
		kong.BindTo(ctx, (*context.Context)(nil)),
	)

	resourceNamePredictor := predictor.NewResourceName(func() (*api.Client, error) {
		// the client for the predictor requires a static token in the client config
		// since dynamic exec config seems to break with some shells during completion.
		// The exact reason for that is unknown.
		apiCluster := defaultAPICluster
		if v, ok := os.LookupEnv("NCTL_API_CLUSTER"); ok {
			apiCluster = v
		}
		c, err := api.New(ctx, apiCluster, "", api.StaticToken(ctx))
		if err != nil {
			return nil, err
		}

		return c, nil
	})

	// completion handling
	completion.Register(
		parser,
		completion.WithPredictor("file", complete.PredictFiles("*")),
		completion.WithPredictor("resource_name", resourceNamePredictor),
	)

	kongCtx, err := parser.Parse(os.Args[1:])
	if err != nil {
		var parseErr *kong.ParseError
		if errors.As(err, &parseErr) {
			// do not error on missing command/argument.
			// Print Usage + friendly message instead.
			if parseErr.Context.Error == nil {
				node := parseErr.Context.Selected()
				if node == nil {
					node = parseErr.Context.Model.Node
				}
				if format.MissingChildren(node) {
					err = format.ExitIfErrorf(err, parseErr.Context.Command())
				}
			}
		}

		parser.FatalIfErrorf(err)
	}

	// handle the login/oidc cmds separately as we should not try to get the
	// API client if we're not logged in.
	command, err := os.Executable()
	if err != nil {
		kongCtx.Fatalf("can not identify executable path of %s: %v", util.NctlName, err)
	}

	if strings.HasPrefix(kongCtx.Command(), format.LoginCommand) {
		tk := &api.DefaultTokenGetter{}
		kongCtx.FatalIfErrorf(nctl.Auth.Login.Run(ctx, command, tk))
		return
	}

	if strings.HasPrefix(kongCtx.Command(), format.LogoutCommand) {
		tk := &api.DefaultTokenGetter{}
		kongCtx.FatalIfErrorf(nctl.Auth.Logout.Run(ctx, command, tk))
		return
	}

	if strings.HasPrefix(kongCtx.Command(), auth.OIDCCmdName) {
		kongCtx.FatalIfErrorf(nctl.Auth.OIDC.Run(ctx, os.Stdout))
		return
	}

	client, err := api.New(ctx, nctl.APICluster, nctl.Project, api.LogClient(ctx, nctl.LogAPIAddress, nctl.LogAPIInsecure))
	if err != nil {
		fmt.Println(err)
		fmt.Printf("\nUnable to get API client, are you logged in?\n\nUse `%s` to login.\n", format.Command().Login())
		os.Exit(1)
	}

	err = kongCtx.Run(ctx, client)
	if err != nil {
		if k8serrors.IsForbidden(err) {
			fmt.Println("nctl: Permission denied: are you part of the org?	")
			
		}else{
			kongCtx.FatalIfErrorf(kongCtx.Run(ctx, client))
		}
	}

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
	if err := merge(result, appCreateKongVars, create.MySQLKongVars(), create.PostgresKongVars()); err != nil {
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

func merge(existing kong.Vars, withs ...kong.Vars) error {
	for _, with := range withs {
		for k, v := range with {
			_, exists := existing[k]
			if exists {
				return fmt.Errorf("variable %q is already in use", k)
			}
			existing[k] = v
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
