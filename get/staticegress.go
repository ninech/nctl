package get

import (
	"context"
	"fmt"
	"strconv"

	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type staticEgressCmd struct {
	resourceCmd
}

func (cmd *staticEgressCmd) Run(ctx context.Context, c *api.Client, get *Cmd) error {
	return get.listPrint(ctx, c, cmd, api.MatchName(cmd.Name))
}

func (cmd *staticEgressCmd) list() client.ObjectList {
	return &networking.StaticEgressList{}
}

func (cmd *staticEgressCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	staticEgressList, ok := list.(*networking.StaticEgressList)
	if !ok {
		return fmt.Errorf("expected %T, got %T", &networking.StaticEgressList{}, list)
	}
	if len(staticEgressList.Items) == 0 {
		return out.notFound(networking.StaticEgressKind, client.Project)
	}

	switch out.Format {
	case full:
		return cmd.printStaticEgresses(staticEgressList.Items, out, true)
	case noHeader:
		return cmd.printStaticEgresses(staticEgressList.Items, out, false)
	case yamlOut:
		return format.PrettyPrintObjects(staticEgressList.GetItems(), format.PrintOpts{Out: &out.Writer})
	case jsonOut:
		return format.PrettyPrintObjects(
			staticEgressList.GetItems(),
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

func (cmd *staticEgressCmd) printStaticEgresses(list []networking.StaticEgress, out *output, header bool) error {
	if header {
		out.writeHeader("NAME", "TARGET", "EGRESS ADDRESS", "DISABLED")
	}

	for _, se := range list {
		out.writeTabRow(
			se.Namespace,
			se.Name,
			se.Spec.ForProvider.Target.Name,
			se.Status.AtProvider.Address,
			strconv.FormatBool(se.Spec.ForProvider.Disabled),
		)
	}

	return out.tabWriter.Flush()
}
