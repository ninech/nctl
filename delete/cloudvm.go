package delete

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
)

type cloudVMCmd struct {
	Name        string        `arg:"" help:"Name of the CloudVM resource."`
	Force       bool          `default:"false" help:"Do not ask for confirmation of deletion."`
	Wait        bool          `default:"true" help:"Wait until CloudVM is fully deleted."`
	WaitTimeout time.Duration `default:"300s" help:"Duration to wait for the deletion. Only relevant if wait is set."`
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
