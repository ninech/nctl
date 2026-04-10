package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	observability "github.com/ninech/apis/observability/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type grafanaCmd struct {
	resourceCmd
	EnableAdminAccess  *bool `help:"Give admin permissions in the Grafana instance."`
	DisableAdminAccess *bool `help:"Revoke admin permissions in the Grafana instance."`
}

func (cmd *grafanaCmd) Run(ctx context.Context, client *api.Client) error {
	grafana := &observability.Grafana{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}

	return cmd.newUpdater(client, grafana, observability.GrafanaKind, func(current resource.Managed) error {
		grafana, ok := current.(*observability.Grafana)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, observability.Grafana{})
		}

		cmd.applyUpdates(grafana)
		return nil
	}).Update(ctx)
}

func (cmd *grafanaCmd) applyUpdates(grafana *observability.Grafana) {
	if cmd.EnableAdminAccess != nil {
		grafana.Spec.ForProvider.EnableAdminAccess = *cmd.EnableAdminAccess
	}
	if cmd.DisableAdminAccess != nil {
		grafana.Spec.ForProvider.EnableAdminAccess = !*cmd.DisableAdminAccess
	}
}
