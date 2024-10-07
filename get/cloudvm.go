package get

import (
	"context"
	"io"
	"text/tabwriter"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type cloudVMCmd struct {
	resourceCmd
	out io.Writer
}

func (cmd *cloudVMCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	cmd.out = defaultOut(cmd.out)

	cloudVMList := &infrastructure.CloudVirtualMachineList{}

	if err := get.list(ctx, client, cloudVMList, matchName(cmd.Name)); err != nil {
		return err
	}

	if len(cloudVMList.Items) == 0 {
		get.printEmptyMessage(cmd.out, infrastructure.CloudVirtualMachineKind, client.Project)
		return nil
	}

	switch get.Output {
	case full:
		return cmd.printCloudVirtualMachineInstances(cloudVMList.Items, get, true)
	case noHeader:
		return cmd.printCloudVirtualMachineInstances(cloudVMList.Items, get, false)
	case yamlOut:
		return format.PrettyPrintObjects(cloudVMList.GetItems(), format.PrintOpts{})
	}

	return nil
}

func (cmd *cloudVMCmd) printCloudVirtualMachineInstances(list []infrastructure.CloudVirtualMachine, get *Cmd, header bool) error {
	w := tabwriter.NewWriter(cmd.out, 0, 0, 4, ' ', 0)

	if header {
		get.writeHeader(w, "NAME", "FQDN", "POWER STATE", "IP ADDRESS")
	}

	for _, cloudvm := range list {
		get.writeTabRow(w, cloudvm.Namespace, cloudvm.Name, cloudvm.Status.AtProvider.FQDN, string(cloudvm.Status.AtProvider.PowerState), cloudvm.Status.AtProvider.IPAddress)
	}

	return w.Flush()
}
