package get

import (
	"context"
	"fmt"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type managedServerCmd struct {
	ResourceCmd
}

func (cmd *managedServerCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	return get.listPrint(ctx, client, cmd, api.MatchName(cmd.Name))
}

func (cmd *managedServerCmd) list() client.ObjectList {
	return &infrastructure.ManagedServerList{}
}

func (cmd *managedServerCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	serverList, ok := list.(*infrastructure.ManagedServerList)
	if !ok {
		return fmt.Errorf("expected %T, got %T", &infrastructure.ManagedServerList{}, list)
	}
	if len(serverList.Items) == 0 {
		return out.notFound(infrastructure.ManagedServerKind, client.Project)
	}

	switch out.Format {
	case full:
		return cmd.printManagedServers(serverList.Items, out, true)
	case noHeader:
		return cmd.printManagedServers(serverList.Items, out, false)
	case yamlOut:
		return format.PrettyPrintObjects(serverList.GetItems(), format.PrintOpts{Out: &out.Writer})
	case jsonOut:
		return format.PrettyPrintObjects(
			serverList.GetItems(),
			format.PrintOpts{
				Out:    &out.Writer,
				Format: format.OutputFormatTypeJSON,
				JSONOpts: format.JSONOutputOptions{
					PrintSingleItem: cmd.Name != "",
				},
			})
	}

	return nil
}

func (cmd *managedServerCmd) printManagedServers(list []infrastructure.ManagedServer, out *output, header bool) error {
	if header {
		out.writeHeader("NAME", "LOCATION", "PLATFORM", "OS", "IP ADDRESSES")
	}

	for _, server := range list {
		obs := server.Status.AtProvider
		out.writeTabRow(
			server.Namespace,
			server.Name,
			string(server.Spec.ForProvider.Location),
			string(obs.Platform),
			string(obs.OS),
			joinOrDash(obs.IPAddresses),
		)
	}

	return out.tabWriter.Flush()
}
