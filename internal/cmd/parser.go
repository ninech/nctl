package cmd

import (
	"context"
	"fmt"
	"io"

	"github.com/alecthomas/kong"
	"github.com/ninech/nctl/internal/apifield"
	"github.com/ninech/nctl/internal/application"
	"github.com/ninech/nctl/internal/bucket"
	"github.com/ninech/nctl/internal/cli"
	"github.com/ninech/nctl/internal/cmd/auth"
	"github.com/ninech/nctl/internal/cmd/create"
	"github.com/ninech/nctl/internal/cmd/logs"
	"github.com/ninech/nctl/internal/cmd/update"
	"github.com/ninech/nctl/internal/completion"
	"github.com/ninech/nctl/internal/serviceconnection"
)

const defaultAPICluster = "nineapis.ch"

// newParser builds the Kong parser for root.
// The given reader and writer are bound as [io.Reader] and [io.Writer],
// so that commands can prompt for input and print output.
// opts are applied after the options of nctl.
func newParser(ctx context.Context, root *rootCommand, version string, r io.Reader, w io.Writer, opts ...kong.Option) (*kong.Kong, error) {
	kongVars, err := kongVariables(version)
	if err != nil {
		return nil, err
	}

	options := []kong.Option{
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
		kong.PostBuild(interpolateFlagPlaceholders(kongVars)),
		kong.PostBuild(apifield.Apply()),
		kongVars,
		kong.BindTo(ctx, (*context.Context)(nil)),
		kong.BindTo(w, (*io.Writer)(nil)),
		kong.BindTo(r, (*io.Reader)(nil)),
	}

	parser, err := kong.New(root, append(options, opts...)...)
	if err != nil {
		return nil, err
	}

	// Completion has to be registered before parsing,
	// so root is still empty here.
	// The predictors resolve the flags they need
	// off the parser model and the command line being completed instead.
	if err := completion.Register(ctx, parser); err != nil {
		return nil, err
	}

	return parser, nil
}

// kongVariables collects all variables which should be passed to kong. It
// checks for variables which would overwrite already existing ones.
func kongVariables(version string) (kong.Vars, error) {
	result := make(kong.Vars)
	result["version"] = version
	result["api_cluster"] = defaultAPICluster
	appKongVars, err := application.KongVars()
	if err != nil {
		return nil, fmt.Errorf("error on application kong vars: %w", err)
	}
	if err := merge(
		result,
		appKongVars,
		create.MySQLKongVars(),
		create.ServiceConnectionKongVars(),
		serviceconnection.KongVars(),
		bucket.KongVars(),
		update.BucketKongVars(),
		auth.LoginKongVars(),
		logs.KongVars(),
	); err != nil {
		return nil, fmt.Errorf("error when merging kong variables: %w", err)
	}

	return result, nil
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
