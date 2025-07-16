package delete

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/api"
)

type serviceConnectionCmd struct {
	resourceCmd
}

func (cmd *serviceConnectionCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	sc := &networking.ServiceConnection{}
	scName := types.NamespacedName{Name: cmd.Name, Namespace: client.Project}
	if err := client.Get(ctx, scName, sc); err != nil {
		return fmt.Errorf("unable to get serviceconnection %q: %w", sc.Name, err)
	}

	return newDeleter(sc, networking.ServiceConnectionKind).deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force)
}
