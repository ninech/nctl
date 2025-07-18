package get

import (
	"context"

	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type serviceConnectionCmd struct {
	resourceCmd
}

func (cmd *serviceConnectionCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	return get.listPrint(ctx, client, cmd, api.MatchName(cmd.Name))
}

func (cmd *serviceConnectionCmd) list() client.ObjectList {
	return &networking.ServiceConnectionList{}
}

func (cmd *serviceConnectionCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	serviceConnectionList := list.(*networking.ServiceConnectionList)
	if len(serviceConnectionList.Items) == 0 {
		out.printEmptyMessage(networking.ServiceConnectionKind, client.Project)
		return nil
	}

	switch out.Format {
	case full:
		return cmd.printServiceConnections(serviceConnectionList.Items, out, true)
	case noHeader:
		return cmd.printServiceConnections(serviceConnectionList.Items, out, false)
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

func (cmd *serviceConnectionCmd) printServiceConnections(list []networking.ServiceConnection, out *output, header bool) error {
	if header {
		out.writeHeader("NAME", "SOURCE", "DESTINATION", "DESTINATION DNS")
	}

	for _, sc := range list {
		out.writeTabRow(sc.Namespace, sc.Name, sc.Spec.ForProvider.Source.Reference.String(), sc.Spec.ForProvider.Destination.String(), sc.Status.AtProvider.DestinationDNS)
	}

	return out.tabWriter.Flush()
}
