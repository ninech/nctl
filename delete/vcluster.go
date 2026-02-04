package delete

import (
	"context"
	"fmt"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/config"
	"k8s.io/apimachinery/pkg/types"
)

type vclusterCmd struct {
	resourceCmd
}

func (cmd *vclusterCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	cluster := &infrastructure.KubernetesCluster{}
	clusterName := types.NamespacedName{Name: cmd.Name, Namespace: client.Project}
	if err := client.Get(ctx, clusterName, cluster); err != nil {
		return fmt.Errorf("unable to get vcluster %q: %w", cluster.Name, err)
	}

	if cluster.Spec.ForProvider.VCluster == nil {
		return fmt.Errorf("supplied cluster %q is not a vcluster", config.ContextName(cluster))
	}

	d := cmd.newDeleter(cluster, "vcluster", cleanup(
		func(client *api.Client) error {
			if err := config.RemoveClusterFromKubeConfig(
				client.KubeconfigPath,
				config.ContextName(cluster),
			); err != nil {
				cmd.Warningf("unable to remove cluster from kubeconfig: %s\n", err)
			}
			return nil
		}))

	if err := d.deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force); err != nil {
		return fmt.Errorf("unable to delete vcluster: %w", err)
	}

	return nil
}
