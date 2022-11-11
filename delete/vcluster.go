package delete

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/auth"
	"k8s.io/apimachinery/pkg/api/errors"
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

	if !vc.Force {
		if !confirmf("do you really want to delete the vcluster %q?", auth.ContextName(cluster)) {
			fmt.Println(" ✗ vcluster deletion canceled")
			return nil
		}
	}

	if err := client.Delete(ctx, cluster); err != nil {
		return fmt.Errorf("unable to delete vcluster %q: %w", auth.ContextName(cluster), err)
	}
	fmt.Println(" ✓ vcluster deletion started")

	if vc.Wait {
		if err := waitForDeletion(ctx, client, cluster); err != nil {
			return fmt.Errorf("error waiting for deletion: %w", err)
		}
	}

	return auth.RemoveClusterFromConfig(client, auth.ContextName(cluster))
}

func confirmf(format string, a ...any) bool {
	var input string

	fmt.Printf("%s [y|n]: ", fmt.Sprintf(format, a...))
	_, err := fmt.Scanln(&input)
	if err != nil {
		panic(err)
	}
	input = strings.ToLower(input)

	if input == "y" || input == "yes" {
		return true
	}
	return false
}

func waitForDeletion(ctx context.Context, client *api.Client, cluster *infrastructure.KubernetesCluster) error {
	spin := spinner.New(spinner.CharSets[7], 100*time.Millisecond)
	spin.Prefix = " "
	spin.Start()
	defer spin.Stop()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			clusterName := types.NamespacedName{Name: cluster.Name, Namespace: cluster.Namespace}
			if err := client.Get(ctx, clusterName, cluster); err != nil {
				if errors.IsNotFound(err) {
					spin.Stop()

					fmt.Println(" ✓ vcluster deleted 🗑")
					return nil
				}
				return fmt.Errorf("unable to get vcluster %q: %w", auth.ContextName(cluster), err)
			}
		case <-ctx.Done():
			spin.Stop()
			return fmt.Errorf("timeout waiting for vcluster")
		}
	}
}
