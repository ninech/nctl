package delete

import (
	"context"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type cloudVMCmd struct {
	resourceCmd
}

func (cmd *cloudVMCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	cloudVM := &infrastructure.CloudVirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: cmd.Name, Namespace: client.Project}}
	return cmd.newDeleter(cloudVM, infrastructure.CloudVirtualMachineKind).deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force)
}
