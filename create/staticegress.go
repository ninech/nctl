package create

import (
	"context"
	"fmt"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	apps "github.com/ninech/apis/apps/v1alpha1"
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type staticEgressCmd struct {
	resourceCmd
	Application string `help:"Name of the target Application." xor:"target"`
	Cluster     string `help:"Name of the target KubernetesCluster (NKE/vCluster)." xor:"target"`
	Disabled    bool   `help:"Create the static egress in disabled state." default:"false"`
}

func (cmd *staticEgressCmd) Run(ctx context.Context, client *api.Client, create *Cmd) error {
	if ok, err := create.applyFile(ctx, cmd.Writer, client); ok {
		return err
	}
	staticEgress, err := cmd.newStaticEgress(client.Project)
	if err != nil {
		return err
	}

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

func (cmd *staticEgressCmd) newStaticEgress(namespace string) (*networking.StaticEgress, error) {
	target, err := cmd.target()
	if err != nil {
		return nil, err
	}

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
				Target:   target,
			},
		},
	}, nil
}

func (cmd *staticEgressCmd) target() (meta.LocalTypedReference, error) {
	switch {
	case cmd.Application != "":
		return meta.LocalTypedReference{
			LocalReference: meta.LocalReference{Name: cmd.Application},
			GroupKind:      metav1.GroupKind{Group: apps.Group, Kind: apps.ApplicationKind},
		}, nil
	case cmd.Cluster != "":
		return meta.LocalTypedReference{
			LocalReference: meta.LocalReference{Name: cmd.Cluster},
			GroupKind:      metav1.GroupKind{Group: infrastructure.Group, Kind: infrastructure.KubernetesClusterKind},
		}, nil
	default:
		return meta.LocalTypedReference{}, fmt.Errorf("missing static egress target")
	}
}
