package create

import (
	"context"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type staticEgressCmd struct {
	resourceCmd
	Application string `help:"Name of the target Application." required:""`
	Disabled    bool   `help:"Create the static egress in disabled state." default:"false"`
}

func (cmd *staticEgressCmd) Run(ctx context.Context, client *api.Client) error {
	staticEgress := cmd.newStaticEgress(client.Project)

	c := cmd.newCreator(client, staticEgress, networking.StaticEgressKind)
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !cmd.Wait {
		return nil
	}

	return c.wait(ctx, waitStage{
		objectList: &networking.StaticEgressList{},
		onResult: func(event watch.Event) (bool, error) {
			if se, ok := event.Object.(*networking.StaticEgress); ok {
				return isAvailable(se), nil
			}
			return false, nil
		},
	})
}

func (cmd *staticEgressCmd) newStaticEgress(namespace string) *networking.StaticEgress {
	name := getName(cmd.Name)

	return &networking.StaticEgress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: networking.StaticEgressSpec{
			ResourceSpec: runtimev1.ResourceSpec{},
			ForProvider: networking.StaticEgressParameters{
				Disabled: cmd.Disabled,
				Target: meta.LocalTypedReference{
					LocalReference: meta.LocalReference{
						Name: cmd.Application,
					},
					GroupKind: metav1.GroupKind{
						Group: apps.Group,
						Kind:  apps.ApplicationKind,
					},
				},
			},
		},
	}
}
