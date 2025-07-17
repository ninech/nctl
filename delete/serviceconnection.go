package delete

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/api"
)

type serviceConnectionCmd struct {
	resourceCmd
}

func (cmd *serviceConnectionCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	sc := &networking.ServiceConnection{ObjectMeta: metav1.ObjectMeta{Name: cmd.Name, Namespace: client.Project}}
	return newDeleter(sc, networking.ServiceConnectionKind).deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force)
}
