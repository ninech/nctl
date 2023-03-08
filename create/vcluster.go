package create

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/briandowns/spinner"
	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/lucasepe/codename"
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/auth"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type vclusterCmd struct {
	Name              string        `arg:"" default:"" help:"Name of the vcluster. A random name is generated if omitted."`
	Location          string        `default:"nine-es34" help:"Location where the vcluster is created."`
	KubernetesVersion string        `default:"1.24" help:"Kubernetes version to use."`
	MinNodes          int           `default:"1" help:"Minimum amount of nodes."`
	MaxNodes          int           `default:"1" help:"Maximum amount of nodes."`
	MachineType       string        `default:"nine-standard-1" help:"Machine type to use for the nodes."`
	NodePoolName      string        `default:"worker" help:"Name of the default node pool in the vcluster."`
	Wait              bool          `default:"true" help:"Wait until vcluster is fully created."`
	WaitTimeout       time.Duration `default:"300s" help:"Duration to wait for vcluster getting ready. Only relevant if wait is set."`
}

func (vc *vclusterCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, vc.WaitTimeout)
	defer cancel()

	if vc.Name == "" {
		vc.Name = codename.Generate(rand.New(rand.NewSource(time.Now().UnixNano())), 0)
	}

	cluster := vc.newCluster(client.Namespace)

	if err := client.Create(ctx, cluster); err != nil {
		return fmt.Errorf("unable to create vcluster: %w", err)
	}

	fmt.Printf(" ✓ created vcluster %q\n", cluster.Name)

	if !vc.Wait {
		return nil
	}

	return waitForCreation(ctx, client, cluster)
}

func waitForCreation(ctx context.Context, client *api.Client, cluster *infrastructure.KubernetesCluster) error {
	fmt.Println(" ✓ waiting for vcluster to be ready ⏳")
	spin := spinner.New(spinner.CharSets[7], 100*time.Millisecond)
	spin.Prefix = " "
	spin.Start()
	defer spin.Stop()

	watch, err := client.Watch(
		ctx, &infrastructure.KubernetesClusterList{},
		runtimeclient.InNamespace(client.Namespace),
		runtimeclient.MatchingFields{"metadata.name": cluster.Name},
	)
	if err != nil {
		return fmt.Errorf("unable to watch vcluster: %w", err)
	}

	for {
		select {
		case res := <-watch.ResultChan():
			if c, ok := res.Object.(*infrastructure.KubernetesCluster); ok {
				if isAvailable(c) {
					watch.Stop()
					spin.Stop()

					fmt.Println(" ✓ vcluster ready 🐧")
					clustercmd := auth.ClusterCmd{Name: auth.ContextName(c), ExecPlugin: true}
					return clustercmd.Run(ctx, client)
				}
			}
		case <-ctx.Done():
			spin.Stop()
			return fmt.Errorf("timeout waiting for vcluster")
		}
	}
}

func (vc *vclusterCmd) newCluster(namespace string) *infrastructure.KubernetesCluster {
	return &infrastructure.KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vc.Name,
			Namespace: namespace,
		},
		Spec: infrastructure.KubernetesClusterSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      vc.Name,
					Namespace: namespace,
				},
			},
			ForProvider: infrastructure.KubernetesClusterParameters{
				VCluster: &infrastructure.VClusterSettings{
					Version: vc.KubernetesVersion,
				},
				Location: meta.LocationName(vc.Location),
				NodePools: []infrastructure.NodePool{
					{
						Name:        vc.NodePoolName,
						MinNodes:    vc.MinNodes,
						MaxNodes:    vc.MaxNodes,
						MachineType: infrastructure.MachineType(vc.MachineType),
					},
				},
			},
		},
	}
}

func isAvailable(cluster *infrastructure.KubernetesCluster) bool {
	return cluster.GetCondition(runtimev1.TypeReady).Reason == runtimev1.ReasonAvailable &&
		cluster.GetCondition(runtimev1.TypeReady).Status == corev1.ConditionTrue &&
		len(cluster.Status.AtProvider.APIEndpoint) != 0
}
