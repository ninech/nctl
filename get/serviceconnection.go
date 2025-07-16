package get

import (
	"context"
	"io"
	"text/tabwriter"

	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type serviceConnectionCmd struct {
	resourceCmd
	out io.Writer
}

func (cmd *serviceConnectionCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	cmd.out = defaultOut(cmd.out)

	serviceConnectionList := &networking.ServiceConnectionList{}

	if err := get.list(ctx, client, serviceConnectionList, api.MatchName(cmd.Name)); err != nil {
		return err
	}

	if len(serviceConnectionList.Items) == 0 {
		get.printEmptyMessage(cmd.out, networking.ServiceConnectionKind, client.Project)
		return nil
	}

	switch get.Output {
	case full:
		return cmd.printServiceConnections(serviceConnectionList.Items, get, true)
	case noHeader:
		return cmd.printServiceConnections(serviceConnectionList.Items, get, false)
	case yamlOut:
		return format.PrettyPrintObjects(serviceConnectionList.GetItems(), format.PrintOpts{})
	case jsonOut:
		return format.PrettyPrintObjects(
			serviceConnectionList.GetItems(),
			format.PrintOpts{
				Format: format.OutputFormatTypeJSON,
				JSONOpts: format.JSONOutputOptions{
					PrintSingleItem: cmd.Name != "",
				},
			})
	}

	return nil
}

func (cmd *serviceConnectionCmd) printServiceConnections(list []networking.ServiceConnection, get *Cmd, header bool) error {
	w := tabwriter.NewWriter(cmd.out, 0, 0, 4, ' ', 0)

	if header {
		get.writeHeader(w, "NAME", "SOURCE", "DESTINATION", "DESTINATION DNS")
	}

	for _, sc := range list {
		get.writeTabRow(w, sc.Namespace, sc.Name, sc.Spec.ForProvider.Source.Reference.String(), sc.Spec.ForProvider.Destination.String(), sc.Status.AtProvider.DestinationDNS)
	}

	return w.Flush()
}
