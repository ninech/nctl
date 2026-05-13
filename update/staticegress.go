package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type staticEgressCmd struct {
	resourceCmd
	Disabled *bool `negatable:"" help:"Enable or disable the static egress."`
}

func (cmd *staticEgressCmd) Run(ctx context.Context, client *api.Client) error {
	staticEgress := &networking.StaticEgress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}

	return cmd.newUpdater(client, staticEgress, networking.StaticEgressKind, func(current resource.Managed) error {
		staticEgress, ok := current.(*networking.StaticEgress)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, networking.StaticEgress{})
		}

		return cmd.applyUpdates(staticEgress)
	}).Update(ctx)
}

func (cmd *staticEgressCmd) applyUpdates(staticEgress *networking.StaticEgress) error {
	if cmd.Disabled != nil {
		staticEgress.Spec.ForProvider.Disabled = *cmd.Disabled
	}
	return nil
}
