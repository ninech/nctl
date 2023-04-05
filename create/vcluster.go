package create

import (
	"context"
	"time"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/auth"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
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
	c := newCreator(vc.newCluster(client.Namespace), "vcluster", &infrastructure.KubernetesClusterList{})
	ctx, cancel := context.WithTimeout(ctx, vc.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx, client); err != nil {
		return err
	}

	if !vc.Wait {
		return nil
	}

	return c.wait(ctx, client, func(event watch.Event) (bool, error) {
		if c, ok := event.Object.(*infrastructure.KubernetesCluster); ok {
			if vc.isAvailable(c) {
				clustercmd := auth.ClusterCmd{Name: auth.ContextName(c), ExecPlugin: true}
				return true, clustercmd.Run(ctx, client)
			}
		}

		return false, nil
	})
}

func (vc *vclusterCmd) isAvailable(cluster *infrastructure.KubernetesCluster) bool {
	return isAvailable(cluster) && len(cluster.Status.AtProvider.APIEndpoint) != 0
}

func (vc *vclusterCmd) newCluster(namespace string) *infrastructure.KubernetesCluster {
	name := getName(vc.Name)
	return &infrastructure.KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: infrastructure.KubernetesClusterSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      name,
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
