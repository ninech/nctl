package delete

import (
	"context"

	observability "github.com/ninech/apis/observability/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type grafanaCmd struct {
	resourceCmd
}

func (cmd *grafanaCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	grafana := &observability.Grafana{ObjectMeta: metav1.ObjectMeta{Name: cmd.Name, Namespace: client.Project}}
	return cmd.newDeleter(grafana, observability.GrafanaKind).deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force)
}
