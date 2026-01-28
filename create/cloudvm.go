package create

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type cloudVMCmd struct {
	resourceCmd
	Location            meta.LocationName                       `default:"nine-es34" help:"Where the CloudVM instance is created."`
	MachineType         string                                  `default:"" help:"Defines the sizing for a particular CloudVM."`
	Hostname            string                                  `default:"" help:"Configures the hostname explicitly. If unset, the name of the resource will be used as the hostname. This does not affect the DNS name."`
	ReverseDNS          string                                  `default:"" help:"Configures the reverse DNS of the CloudVM."`
	PowerState          infrastructure.VirtualMachinePowerState `default:"on" help:"Specify the initial power state of the CloudVM. Set to off to not start the VM after creation."`
	OS                  infrastructure.OperatingSystem          `default:"" help:"Operating system to use to boot the VM. Available options: ${cloudvm_os_flavors}"`
	BootDiskSize        *resource.Quantity                      `default:"20Gi" help:"Configures the size of the boot disk."`
	Disks               map[string]resource.Quantity            `default:"" help:"Additional disks to mount to the machine."`
	PublicKeys          []string                                `default:"" help:"SSH public keys to connect to the CloudVM as root. The keys are expected to be in SSH format as defined in RFC4253. Immutable after creation."`
	PublicKeysFromFiles []*os.File                              `default:"" completion-predictor:"file" help:"SSH public key files to connect to the VM as root. The keys are expected to be in SSH format as defined in RFC4253. Immutable after creation."`
	CloudConfig         string                                  `default:"" help:"Pass custom cloud config data (https://cloudinit.readthedocs.io/en/latest/topics/format.html#cloud-config-data) to the cloud VM. If a CloudConfig is passed, the PublicKey parameter is ignored. Immutable after creation."`
	CloudConfigFromFile *os.File                                `default:"" completion-predictor:"file" help:"Pass custom cloud config data (https://cloudinit.readthedocs.io/en/latest/topics/format.html#cloud-config-data) from a file. Takes precedence. If a CloudConfig is passed, the PublicKey parameter is ignored. Immutable after creation."`
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

	fmt.Printf("\nYour Cloud VM %s is now available, you can now connect with:\n  ssh root@%s\n\n", cloudVM.Name, cloudVM.Status.AtProvider.FQDN)

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
				Location:    cmd.Location,
				MachineType: infrastructure.NewMachineType(cmd.MachineType),
				Hostname:    cmd.Hostname,
				PowerState:  cmd.PowerState,
				OS:          infrastructure.CloudVirtualMachineOS(cmd.OS),
				PublicKeys:  cmd.PublicKeys,
				CloudConfig: cmd.CloudConfig,
				ReverseDNS:  cmd.ReverseDNS,
			},
		},
	}

	if len(cmd.PublicKeysFromFiles) != 0 {
		cloudVM.Spec.ForProvider.PublicKeys = cmd.PublicKeys
		var keys []string
		for _, file := range cmd.PublicKeysFromFiles {
			if file == nil {
				continue
			}

			b, err := io.ReadAll(file)
			if err != nil {
				return nil, fmt.Errorf("error reading public keys file: %w", err)
			}
			keys = append(keys, string(b))
		}
		cloudVM.Spec.ForProvider.PublicKeys = keys
	}

	if cmd.CloudConfigFromFile != nil {
		b, err := io.ReadAll(cmd.CloudConfigFromFile)
		if err != nil {
			return nil, fmt.Errorf("error reading cloudconfig file: %w", err)
		}
		cloudVM.Spec.ForProvider.CloudConfig = string(b)
	}

	if len(cmd.Disks) != 0 {
		disks := []infrastructure.Disk{}
		for name, size := range cmd.Disks {
			disks = append(disks, infrastructure.Disk{Name: name, Size: size})
		}
		cloudVM.Spec.ForProvider.Disks = disks
	}

	if cmd.BootDiskSize != nil {
		cloudVM.Spec.ForProvider.BootDisk = &infrastructure.Disk{Name: "root", Size: *cmd.BootDiskSize}
	}

	return cloudVM, nil
}

// ApplicationKongVars returns all variables which are used in the application
// create command
func CloudVMKongVars() kong.Vars {
	result := make(kong.Vars)
	result["cloudvm_os_flavors"] = strings.Join(stringSlice(infrastructure.CloudVirtualMachineOperatingSystems), ", ")

	return result
}
