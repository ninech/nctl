package create

import (
	"context"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	observability "github.com/ninech/apis/observability/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type grafanaCmd struct {
	resourceCmd
	EnableAdminAccess bool `help:"Give admin permissions in the Grafana instance."`
	AllowLocalUsers   bool `help:"Allow local Grafana users to sign in by disabling the automatic redirect to the OAuth sign-in page."`
}

func (cmd *grafanaCmd) Run(ctx context.Context, client *api.Client) error {
	grafana := cmd.newGrafana(client.Project)

	c := cmd.newCreator(client, grafana, observability.GrafanaKind)
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !cmd.Wait {
		return nil
	}

	return c.wait(ctx, waitStage{
		objectList: &observability.GrafanaList{},
		onResult: func(event watch.Event) (bool, error) {
			if g, ok := event.Object.(*observability.Grafana); ok {
				return isAvailable(g), nil
			}
			return false, nil
		},
	})
}

func (cmd *grafanaCmd) newGrafana(namespace string) *observability.Grafana {
	name := getName(cmd.Name)

	return &observability.Grafana{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: observability.GrafanaSpec{
			ResourceSpec: runtimev1.ResourceSpec{},
			ForProvider: observability.GrafanaParameters{
				EnableAdminAccess: cmd.EnableAdminAccess,
				AllowLocalUsers:   cmd.AllowLocalUsers,
			},
		},
	}
}
