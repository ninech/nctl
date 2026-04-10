package get

import (
	"context"
	"fmt"
	"strconv"

	observability "github.com/ninech/apis/observability/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type grafanaCmd struct {
	resourceCmd
}

func (cmd *grafanaCmd) Run(ctx context.Context, c *api.Client, get *Cmd) error {
	return get.listPrint(ctx, c, cmd, api.MatchName(cmd.Name))
}

func (cmd *grafanaCmd) list() client.ObjectList {
	return &observability.GrafanaList{}
}

func (cmd *grafanaCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	grafanaList, ok := list.(*observability.GrafanaList)
	if !ok {
		return fmt.Errorf("expected %T, got %T", &observability.GrafanaList{}, list)
	}
	if len(grafanaList.Items) == 0 {
		return out.notFound(observability.GrafanaKind, client.Project)
	}

	switch out.Format {
	case full:
		return cmd.printGrafanaInstances(grafanaList.Items, out, true)
	case noHeader:
		return cmd.printGrafanaInstances(grafanaList.Items, out, false)
	case yamlOut:
		return format.PrettyPrintObjects(grafanaList.GetItems(), format.PrintOpts{Out: &out.Writer})
	case jsonOut:
		return format.PrettyPrintObjects(
			grafanaList.GetItems(),
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

func (cmd *grafanaCmd) printGrafanaInstances(list []observability.Grafana, out *output, header bool) error {
	if header {
		out.writeHeader("NAME", "URL", "ADMIN ACCESS")
	}

	for _, g := range list {
		out.writeTabRow(
			g.Namespace,
			g.Name,
			g.Status.AtProvider.URL,
			strconv.FormatBool(g.Spec.ForProvider.EnableAdminAccess),
		)
	}

	return out.tabWriter.Flush()
}
