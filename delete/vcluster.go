package delete

import (
	"context"
	"fmt"
	"time"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/auth"
	"k8s.io/apimachinery/pkg/types"
)

type vclusterCmd struct {
	Name        string        `arg:"" help:"Name of the vcluster."`
	Force       bool          `default:"false" help:"Do not ask for confirmation of deletion."`
	Wait        bool          `default:"true" help:"Wait until vcluster is fully deleted"`
	WaitTimeout time.Duration `default:"300s" help:"Duration to wait for the deletion. Only relevant if wait is set."`
}

func (vc *vclusterCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, vc.WaitTimeout)
	defer cancel()

	cluster := &infrastructure.KubernetesCluster{}
	clusterName := types.NamespacedName{Name: vc.Name, Namespace: client.Namespace}
	if err := client.Get(ctx, clusterName, cluster); err != nil {
		return fmt.Errorf("unable to get vcluster %q: %w", cluster.Name, err)
	}

	if cluster.Spec.ForProvider.VCluster == nil {
		return fmt.Errorf("supplied cluster %q is not a vcluster", auth.ContextName(cluster))
	}

	d := newDeleter(cluster, "vcluster", func(client *api.Client) error {
		return auth.RemoveClusterFromConfig(client, auth.ContextName(cluster))
	})

	if err := d.deleteResource(ctx, client, vc.WaitTimeout, vc.Wait, vc.Force); err != nil {
		return fmt.Errorf("unable to delete vcluster: %w", err)
	}

	return nil
}
