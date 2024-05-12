package create

import (
	"context"
	"fmt"
	"os"
	"time"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type cloudVMCmd struct {
	Name                string            `arg:"" default:"" help:"Name of the CloudVM instance. A random name is generated if omitted."`
	Location            string            `default:"nine-es34" help:"Location where the CloudVM instance is created."`
	MachineType         string            `default:"" help:"MachineType defines the sizing for a particular cloud vm."`
	Hostname            string            `default:"" help:"Hostname allows to set the hostname explicitly. If unset, the name of the resource will be used as the hostname. This does not affect the DNS name."`
	PowerState          string            `default:"on" help:"PowerState specifies the power state of the cloud VM. A value of On turns the VM on, shutdown sends an ACPI signal to the VM to perform a clean shutdown and off forces the power off immediately."`
	OS                  string            `default:"" help:"OS which should be used to boot the VM."`
	BootDiskSize        string            `default:"20Gi" help:"BootDiskSize that will be used to boot the VM from."`
	Disks               map[string]string `default:"" help:"Disks specifies which additional disks to mount to the machine."`
	PublicKeys          []string          `default:"" help:"PublicKeys specifies the SSH Public Keys that can be used to connect to the VM as root. The keys are expected to be in SSH format as defined in RFC4253. Immutable after creation."`
	PublicKeysFromFiles []string          `default:"" predictor:"file" help:"CloudConfig via file. Has precedence over args. PublicKeys specifies the SSH Public Keys that can be used to connect to the VM as root. The keys are expected to be in SSH format as defined in RFC4253. Immutable after creation."`
	CloudConfig         string            `default:"" help:"CloudConfig allows to pass custom cloud config data (https://cloudinit.readthedocs.io/en/latest/topics/format.html#cloud-config-data) to the cloud VM. If a CloudConfig is passed, the PublicKey parameter is ignored. Immutable after creation."`
	CloudConfigFromFile string            `default:"" predictor:"file" help:"CloudConfig via file. Has precedence over args. CloudConfig allows to pass custom cloud config data (https://cloudinit.readthedocs.io/en/latest/topics/format.html#cloud-config-data) to the cloud VM. If a CloudConfig is passed, the PublicKey parameter is ignored. Immutable after creation."`
	Wait                bool              `default:"true" help:"Wait until CloudVM is created."`
	WaitTimeout         time.Duration     `default:"600s" help:"Duration to wait for CloudVM getting ready. Only relevant if --wait is set."`
}

func (cmd *cloudVMCmd) Run(ctx context.Context, client *api.Client) error {
	cloudVM, err := cmd.newCloudVM(client.Project)
	if err != nil {
		return err
	}

	c := newCreator(client, cloudVM, infrastructure.CloudVirtualMachineKind)
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !cmd.Wait {
		return nil
	}

	if err := c.wait(ctx, waitStage{
		objectList: &infrastructure.CloudVirtualMachineList{},
		onResult: func(event watch.Event) (bool, error) {
			if c, ok := event.Object.(*infrastructure.CloudVirtualMachine); ok {
				cloudVM = c
				return isAvailable(c), nil
			}
			return false, nil
		}},
	); err != nil {
		return err
	}

	fmt.Printf("\n Your Cloud VM %s is now available, you can now connect with: ssh %s\n\n", cloudVM.Name, cloudVM.Status.AtProvider.FQDN)

	return nil
}

func (cmd *cloudVMCmd) newCloudVM(namespace string) (*infrastructure.CloudVirtualMachine, error) {
	name := getName(cmd.Name)

	cloudVM := &infrastructure.CloudVirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: infrastructure.CloudVirtualMachineSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      "cloudvirtualmachine-" + name,
					Namespace: namespace,
				},
			},
			ForProvider: infrastructure.CloudVirtualMachineParameters{
				Location:    meta.LocationName(cmd.Location),
				MachineType: infrastructure.MachineType(cmd.MachineType),
				Hostname:    cmd.Hostname,
				PowerState:  infrastructure.VirtualMachinePowerState(cmd.PowerState),
				OS:          infrastructure.CloudVirtualMachineOS(infrastructure.OperatingSystem(cmd.OS)),
				PublicKeys:  cmd.PublicKeys,
				CloudConfig: cmd.CloudConfig,
			},
		},
	}

	if len(cmd.PublicKeysFromFiles) != 0 {
		cloudVM.Spec.ForProvider.PublicKeys = cmd.PublicKeys
		var keys []string
		for _, file := range cmd.PublicKeysFromFiles {
			b, err := os.ReadFile(file)
			if err != nil {
				return nil, fmt.Errorf("error reading cloudconfig file %q: %w", cmd.PublicKeysFromFiles, err)
			}
			keys = append(keys, string(b))
		}
		cloudVM.Spec.ForProvider.PublicKeys = keys
	}

	if len(cmd.CloudConfigFromFile) != 0 {
		b, err := os.ReadFile(cmd.CloudConfigFromFile)
		if err != nil {
			return nil, fmt.Errorf("error reading cloudconfig file %q: %w", cmd.CloudConfigFromFile, err)
		}
		cloudVM.Spec.ForProvider.CloudConfig = string(b)
	}

	if len(cmd.Disks) != 0 {
		disks := []infrastructure.Disk{}
		for name, size := range cmd.Disks {
			q, err := resource.ParseQuantity(size)
			if err != nil {
				return nil, fmt.Errorf("error parsing disk size %q: %w", size, err)
			}
			disks = append(disks, infrastructure.Disk{Name: name, Size: q})
		}
		cloudVM.Spec.ForProvider.Disks = disks
	}

	if len(cmd.BootDiskSize) != 0 {
		q, err := resource.ParseQuantity(cmd.BootDiskSize)
		if err != nil {
			return cloudVM, fmt.Errorf("error parsing disk size %q: %w", cmd.BootDiskSize, err)
		}
		cloudVM.Spec.ForProvider.BootDisk = &infrastructure.Disk{Name: "root", Size: q}

	}

	return cloudVM, nil
}
