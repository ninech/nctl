package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/create"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type serviceConnectionCmd struct {
	resourceCmd
	KubernetesClusterOptions create.KubernetesClusterOptions `embed:"" prefix:"source-"`
}

func (cmd *serviceConnectionCmd) Run(ctx context.Context, client *api.Client) error {
	sc := &networking.ServiceConnection{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}

	return cmd.newUpdater(client, sc, networking.ServiceConnectionKind, func(current resource.Managed) error {
		sc, ok := current.(*networking.ServiceConnection)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, networking.ServiceConnection{})
		}

		return cmd.applyUpdates(sc)
	}).Update(ctx)
}

func (cmd *serviceConnectionCmd) applyUpdates(sc *networking.ServiceConnection) error {
	sc.Spec.ForProvider.Source.KubernetesClusterOptions = cmd.KubernetesClusterOptions.APIType()

	return nil
}
