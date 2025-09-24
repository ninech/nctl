package update

import (
	"context"
	"fmt"
	"os"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	res "k8s.io/apimachinery/pkg/api/resource"
)

type cloudVMCmd struct {
	resourceCmd
	MachineType               string            `placeholder:"nine-standard-1" help:"Defines the sizing for a particular CloudVM."`
	Hostname                  string            `placeholder:"" help:"Configures the hostname explicitly. If unset, the name of the resource will be used as the hostname. This does not affect the DNS name."`
	ReverseDNS                string            `placeholder:"" help:"Allows to set the reverse DNS of the CloudVM."`
	OS                        string            `placeholder:"ubuntu22.04" help:"OS which should be used to boot the VM."`
	BootDiskSize              string            `placeholder:"20Gi" help:"Configures the size of the boot disk."`
	Disks                     map[string]string `placeholder:"{}" help:"Additional disks to mount to the machine."`
	On                        *bool             `help:"Turns the CloudVM on."`
	Off                       *bool             `help:"Turns the CloudVM off immediately."`
	Shutdown                  *bool             `help:"Shuts down the CloudVM via ACPI."`
	BootRescue                *bool             `help:"Boot CloudVM into a live rescue environment."`
	RescuePublicKeys          []string          `placeholder:"ssh-ed25519" help:"SSH public keys that can be used to connect to the CloudVM while booted into rescue. The keys are expected to be in SSH format as defined in RFC4253."`
	RescuePublicKeysFromFiles []string          `placeholder:"~/.ssh/id_ed25519.pub" predictor:"file" help:"SSH public key files that can be used to connect to the CloudVM while booted into rescue. The keys are expected to be in SSH format as defined in RFC4253."`
}

func (cmd *cloudVMCmd) Run(ctx context.Context, client *api.Client) error {
	cloudvm := &infrastructure.CloudVirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}

	if err := newUpdater(client, cloudvm, infrastructure.CloudVirtualMachineKind, func(current resource.Managed) error {
		cloudvm, ok := current.(*infrastructure.CloudVirtualMachine)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, infrastructure.CloudVirtualMachine{})
		}

		return cmd.applyUpdates(cloudvm)
	}).Update(ctx); err != nil {
		return err
	}

	if cmd.BootRescue != nil && *cmd.BootRescue {
		fmt.Println("Booting CloudVM into rescue mode. It can take a few minutes for the VM to be reachable.")
	}

	return nil
}

func (cmd *cloudVMCmd) applyUpdates(cloudVM *infrastructure.CloudVirtualMachine) error {
	if cmd.MachineType != "" {
		cloudVM.Spec.ForProvider.MachineType = infrastructure.NewMachineType(cmd.MachineType)
	}

	if cmd.Hostname != "" {
		cloudVM.Spec.ForProvider.Hostname = cmd.Hostname
	}

	if cmd.OS != "" {
		cloudVM.Spec.ForProvider.OS = infrastructure.CloudVirtualMachineOS(cmd.OS)
	}

	if cmd.BootDiskSize != "" {
		q, err := res.ParseQuantity(cmd.BootDiskSize)
		if err != nil {
			return fmt.Errorf("error parsing disk size %q: %w", cmd.BootDiskSize, err)
		}
		cloudVM.Spec.ForProvider.BootDisk = &infrastructure.Disk{Name: cmd.BootDiskSize, Size: q}
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

	if cmd.BootRescue != nil {
		if cloudVM.Spec.ForProvider.Rescue == nil {
			cloudVM.Spec.ForProvider.Rescue = &infrastructure.CloudVirtualMachineRescue{Enabled: *cmd.BootRescue}
		} else {
			cloudVM.Spec.ForProvider.Rescue.Enabled = *cmd.BootRescue
		}
	}

	if len(cmd.RescuePublicKeysFromFiles) != 0 {
		var keys []string
		for _, file := range cmd.RescuePublicKeysFromFiles {
			b, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("error reading public key file %q: %w", cmd.RescuePublicKeysFromFiles, err)
			}
			keys = append(keys, string(b))
		}
		if cloudVM.Spec.ForProvider.Rescue == nil {
			cloudVM.Spec.ForProvider.Rescue = &infrastructure.CloudVirtualMachineRescue{PublicKeys: keys}
		} else {
			cloudVM.Spec.ForProvider.Rescue.PublicKeys = keys
		}
	}

	if cmd.ReverseDNS != "" {
		cloudVM.Spec.ForProvider.ReverseDNS = cmd.ReverseDNS
	}

	return nil
}
