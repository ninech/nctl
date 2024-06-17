package delete

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
)

type cloudVMCmd struct {
	resourceCmd
}

func (cmd *cloudVMCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	cloudVM := &infrastructure.CloudVirtualMachine{}
	cloudVMName := types.NamespacedName{Name: cmd.Name, Namespace: client.Project}
	if err := client.Get(ctx, cloudVMName, cloudVM); err != nil {
		return fmt.Errorf("unable to get cloud virtual machine %q: %w", cloudVM.Name, err)
	}

	return newDeleter(cloudVM, infrastructure.CloudVirtualMachineKind).deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force)
}
