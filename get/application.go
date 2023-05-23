package get

import (
	"context"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
)

type applicationsCmd struct {
	Name string `arg:"" help:"Name of the Application to get. If omitted all in the namespace will be listed." default:""`
	out  io.Writer
}

func (cmd *applicationsCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	appList := &apps.ApplicationList{}

	if err := get.list(ctx, client, appList, matchName(cmd.Name)); err != nil {
		return err
	}

	if len(appList.Items) == 0 {
		printEmptyMessage(apps.ApplicationKind, client.Namespace)
		return nil
	}

	out := cmd.out
	if out == nil {
		out = os.Stdout
	}

	switch get.Output {
	case full:
		return printApplication(appList.Items, get, out, true)
	case noHeader:
		return printApplication(appList.Items, get, out, false)
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

func join(list []string) string {
	if len(list) == 0 {
		return "none"
	}
	return strings.Join(list, ",")
}
