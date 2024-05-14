package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	res "k8s.io/apimachinery/pkg/api/resource"
)

type cloudVMCmd struct {
	Name        string            `arg:"" help:"Name of the CloudVM instance to update."`
	MachineType string            `placeholder:"nine-standard-1" help:"MachineType defines the sizing for a particular cloud vm."`
	Hostname    string            `placeholder:"" help:"Hostname allows to set the hostname explicitly. If unset, the name of the resource will be used as the hostname. This does not affect the DNS name."`
	OS          string            `placeholder:"ubuntu22.04" help:"OS which should be used to boot the VM."`
	BootDisk    map[string]string `placeholder:"{name:\"root\",size:\"20Gi\"}" help:"BootDisk that will be used to boot the VM from. Needs to be in the following format: {name:\"<name>\",size:\"<size>Gi\"}"`
	Disks       map[string]string `placeholder:"{}" help:"Disks specifies which additional disks to mount to the machine."`
	On          *bool             `placeholder:"false" help:"Turns the cloudvirtualmachine on"`
	Off         *bool             `placeholder:"false" help:"Turns the cloudvirtualmachine off"`
	Shutdown    *bool             `placeholder:"false" help:"Shuts off the cloudvirtualmachine"`
}

func (cmd *cloudVMCmd) Run(ctx context.Context, client *api.Client) error {
	cloudvm := &infrastructure.CloudVirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}

	return newUpdater(client, cloudvm, infrastructure.CloudVirtualMachineKind, func(current resource.Managed) error {
		cloudvm, ok := current.(*infrastructure.CloudVirtualMachine)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, infrastructure.CloudVirtualMachine{})
		}

		return cmd.applyUpdates(cloudvm)
	}).Update(ctx)
}

func (cmd *cloudVMCmd) applyUpdates(cloudVM *infrastructure.CloudVirtualMachine) error {
	if cmd.MachineType != "" {
		cloudVM.Spec.ForProvider.MachineType = infrastructure.MachineType(cmd.MachineType)
	}

	if cmd.Hostname != "" {
		cloudVM.Spec.ForProvider.Hostname = cmd.Hostname
	}

	if cmd.OS != "" {
		cloudVM.Spec.ForProvider.OS = infrastructure.CloudVirtualMachineOS(cmd.OS)
	}

	if len(cmd.BootDisk) != 0 {
		if len(cmd.BootDisk) > 1 {
			return fmt.Errorf("boot disk can only have one entry but got %q", cmd.BootDisk)
		}
		for name, size := range cmd.BootDisk {
			q, err := res.ParseQuantity(size)
			if err != nil {
				return fmt.Errorf("error parsing disk size %q: %w", size, err)
			}
			cloudVM.Spec.ForProvider.BootDisk = &infrastructure.Disk{Name: name, Size: q}
		}
	}

	if len(cmd.Disks) != 0 {
		disks := []infrastructure.Disk{}
		for name, size := range cmd.Disks {
			q, err := res.ParseQuantity(size)
			if err != nil {
				return fmt.Errorf("error parsing disk size %q: %w", size, err)
			}
			disks = append(disks, infrastructure.Disk{Name: name, Size: q})
		}
		cloudVM.Spec.ForProvider.Disks = disks
	}

	if cmd.Off != nil {
		cloudVM.Spec.ForProvider.PowerState = infrastructure.VirtualMachinePowerState("off")
	}

	if cmd.Shutdown != nil {
		cloudVM.Spec.ForProvider.PowerState = infrastructure.VirtualMachinePowerState("shutdown")
	}

	if cmd.On != nil {
		cloudVM.Spec.ForProvider.PowerState = infrastructure.VirtualMachinePowerState("on")
	}

	return nil
}
