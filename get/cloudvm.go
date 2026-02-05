package get

import (
	"context"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type cloudVMCmd struct {
	resourceCmd
}

func (cmd *cloudVMCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	return get.listPrint(ctx, client, cmd, api.MatchName(cmd.Name))
}

func (cmd *cloudVMCmd) list() client.ObjectList {
	return &infrastructure.CloudVirtualMachineList{}
}

func (cmd *cloudVMCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	cloudVMList := list.(*infrastructure.CloudVirtualMachineList)

	if len(cloudVMList.Items) == 0 {
		return out.notFound(infrastructure.CloudVirtualMachineKind, client.Project)
	}

	switch out.Format {
	case full:
		return cmd.printCloudVirtualMachineInstances(cloudVMList.Items, out, true)
	case noHeader:
		return cmd.printCloudVirtualMachineInstances(cloudVMList.Items, out, false)
	case yamlOut:
		return format.PrettyPrintObjects(cloudVMList.GetItems(), format.PrintOpts{Out: &out.Writer})
	case jsonOut:
		return format.PrettyPrintObjects(
			cloudVMList.GetItems(),
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

func (cmd *cloudVMCmd) printCloudVirtualMachineInstances(list []infrastructure.CloudVirtualMachine, out *output, header bool) error {
	if header {
		out.writeHeader("NAME", "LOCATION", "FQDN", "POWER STATE", "IP ADDRESS")
	}

	for _, vm := range list {
		out.writeTabRow(vm.Namespace, vm.Name, string(vm.Spec.ForProvider.Location), vm.Status.AtProvider.FQDN, string(vm.Status.AtProvider.PowerState), vm.Status.AtProvider.IPAddress)
	}

	return out.tabWriter.Flush()
}
