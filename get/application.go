package get

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/hashicorp/go-multierror"
	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/format"
)

type applicationsCmd struct {
	Name                 string `arg:"" help:"Name of the Application to get. If omitted all in the project will be listed." default:""`
	BasicAuthCredentials bool   `help:"Show the basic auth credentials of the application."`
	out                  io.Writer
}

func (cmd *applicationsCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	appList := &apps.ApplicationList{}

	if err := get.list(ctx, client, appList, matchName(cmd.Name)); err != nil {
		return err
	}

	if len(appList.Items) == 0 {
		printEmptyMessage(cmd.out, apps.ApplicationKind, client.Project)
		return nil
	}

	if cmd.BasicAuthCredentials {
		creds, err := gatherCredentials(ctx, appList.Items, client)
		if len(creds) == 0 {
			fmt.Fprintf(defaultOut(cmd.out), "no application with basic auth enabled found\n")
			return err
		}
		if printErr := printCredentials(creds, get, defaultOut(cmd.out)); printErr != nil {
			err = multierror.Append(err, printErr)
		}
		return err
	}

	switch get.Output {
	case full:
		return printApplication(appList.Items, get, defaultOut(cmd.out), true)
	case noHeader:
		return printApplication(appList.Items, get, defaultOut(cmd.out), false)
	case yamlOut:
		return format.PrettyPrintObjects(appList.GetItems(), format.PrintOpts{Out: defaultOut(cmd.out)})
	}

	return nil
}

func printApplication(apps []apps.Application, get *Cmd, out io.Writer, header bool) error {
	w := tabwriter.NewWriter(out, 0, 0, 4, ' ', 0)

	if header {
		get.writeHeader(w, "NAME", "HOSTS", "UNVERIFIED_HOSTS")
	}

	for _, app := range apps {
		verifiedHosts := append(util.VerifiedAppHosts(&app), app.Status.AtProvider.CNAMETarget)
		unverifiedHosts := util.UnverifiedAppHosts(&app)

		get.writeTabRow(w, app.Namespace, app.Name, join(verifiedHosts), join(unverifiedHosts))
	}

	return w.Flush()
}

func printCredentials(creds []appCredentials, get *Cmd, out io.Writer) error {
	if get.Output == yamlOut {
		return format.PrettyPrintResource(creds, format.PrintOpts{Out: out})
	}
	return printCredentialsTabRow(creds, get, out)
}

func printCredentialsTabRow(creds []appCredentials, get *Cmd, out io.Writer) error {
	w := tabwriter.NewWriter(out, 0, 0, 4, ' ', 0)

	if get.Output == full {
		get.writeHeader(w, "NAME", "USERNAME", "PASSWORD")
	}

	for _, cred := range creds {
		get.writeTabRow(w, cred.Project, cred.Application, cred.Username, cred.Password)
	}

	return w.Flush()
}

type appCredentials struct {
	Application string `yaml:"application"`
	Project     string `yaml:"project"`
	util.BasicAuth
}

func gatherCredentials(ctx context.Context, items []apps.Application, c *api.Client) ([]appCredentials, error) {
	var resultErrors error
	creds := []appCredentials{}
	for _, app := range items {
		app := app
		if app.Status.AtProvider.BasicAuthSecret == nil {
			// the app has no basic auth configured so we skip it
			// in the output
			continue
		}
		basicAuth, err := util.NewBasicAuthFromSecret(
			ctx,
			app.Status.AtProvider.BasicAuthSecret.InNamespace(&app),
			c,
		)
		if err != nil {
			resultErrors = multierror.Append(
				resultErrors,
				fmt.Errorf("can not gather credentials for application %q: %w", app.Name, err),
			)
			continue
		}
		creds = append(creds, appCredentials{Application: app.Name, Project: app.Namespace, BasicAuth: *basicAuth})
	}
	return creds, resultErrors
}

func join(list []string) string {
	if len(list) == 0 {
		return "none"
	}
	return strings.Join(list, ",")
}
