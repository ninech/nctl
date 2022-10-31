package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/auth"
	"github.com/ninech/nctl/get"
	"github.com/posener/complete"
	"github.com/willabides/kongplete"
)

type flags struct {
	Namespace  string `help:"Limit commands to a namespace." short:"n"`
	APICluster string `help:"Context name of the API cluster." default:"nineapis.ch"`
}

type rootCommand struct {
	flags
	Get         get.Cmd                      `cmd:"" help:"Get resource."`
	Auth        auth.Cmd                     `cmd:"" help:"Authenticate with resource."`
	Completions kongplete.InstallCompletions `cmd:"" help:"Print shell completions."`
}

func main() {
	nctl := &rootCommand{}
	parser := kong.Must(nctl, kong.Name("nctl"), kong.Description("Interact with Nine API resources."), kong.UsageOnError())

	// completion handling
	kongplete.Complete(parser, kongplete.WithPredictor("file", complete.PredictFiles("*")))

	ctx, err := parser.Parse(os.Args[1:])
	parser.FatalIfErrorf(err)

	// handle the login/oidc cmds separately as we should not try to get the
	// API client if we're not logged in.
	if strings.HasPrefix(ctx.Command(), auth.LoginCmdName) {
		ctx.FatalIfErrorf(nctl.Auth.Login.Run(ctx.Model.Name))
		return
	}

	if strings.HasPrefix(ctx.Command(), auth.OIDCCmdName) {
		ctx.FatalIfErrorf(nctl.Auth.OIDC.Run())
		return
	}

	client, err := api.New(nctl.APICluster, nctl.Namespace)
	if err != nil {
		fmt.Println(err)
		fmt.Printf("\nUnable to get API client, are you logged in?\n\nUse `%s %s` to login.\n", ctx.Model.Name, auth.LoginCmdName)
		os.Exit(1)
	}

	ctx.FatalIfErrorf(ctx.Run(client))
}
