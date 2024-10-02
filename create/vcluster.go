package create

import (
	"context"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/config"
	"github.com/ninech/nctl/auth"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type vclusterCmd struct {
	resourceCmd
	Location          string `default:"nine-es34" help:"Location where the vcluster is created."`
	KubernetesVersion string `default:"" help:"Kubernetes version to use. API default will be used if not specified."`
	MinNodes          int    `default:"1" help:"Minimum amount of nodes."`
	MaxNodes          int    `default:"1" help:"Maximum amount of nodes."`
	MachineType       string `default:"nine-standard-1" help:"Machine type to use for the nodes."`
	NodePoolName      string `default:"worker" help:"Name of the default node pool in the vcluster."`
}

func (vc *vclusterCmd) Run(ctx context.Context, client *api.Client) error {
	cluster := vc.newCluster(client.Project)
	c := newCreator(client, cluster, "vcluster")
	ctx, cancel := context.WithTimeout(ctx, vc.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !vc.Wait {
		return nil
	}

	if err := c.wait(ctx, waitStage{
		objectList: &infrastructure.KubernetesClusterList{},
		onResult: func(event watch.Event) (bool, error) {
			if c, ok := event.Object.(*infrastructure.KubernetesCluster); ok {
				return vc.isAvailable(c), nil
			}
			return false, nil
		}},
	); err != nil {
		return err
	}

	clustercmd := auth.ClusterCmd{Name: config.ContextName(cluster), ExecPlugin: true}
	return clustercmd.Run(ctx, client)
}

func (vc *vclusterCmd) isAvailable(cluster *infrastructure.KubernetesCluster) bool {
	return isAvailable(cluster) && len(cluster.Status.AtProvider.APIEndpoint) != 0
}

func (vc *vclusterCmd) newCluster(project string) *infrastructure.KubernetesCluster {
	name := getName(vc.Name)
	return &infrastructure.KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		Spec: infrastructure.KubernetesClusterSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      name,
					Namespace: project,
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
