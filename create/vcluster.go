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
	Location          meta.LocationName `default:"nine-es34" help:"Where the vCluster is created."`
	KubernetesVersion string            `default:"" help:"Kubernetes version to use. The API default will be used if not specified."`
	MinNodes          int               `default:"1" help:"Minimum amount of nodes."`
	MaxNodes          int               `default:"1" help:"Maximum amount of nodes."`
	MachineType       string            `default:"nine-standard-1" help:"Machine type to use for the vCluster nodes."`
	NodePoolName      string            `default:"worker" help:"Name of the default node pool in the vCluster."`
}

func (cmd *vclusterCmd) Run(ctx context.Context, client *api.Client) error {
	cluster := cmd.newCluster(client.Project)
	c := cmd.newCreator(client, cluster, "vcluster")
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !cmd.Wait {
		return nil
	}

	if err := c.wait(ctx, waitStage{
		objectList: &infrastructure.KubernetesClusterList{},
		onResult: func(event watch.Event) (bool, error) {
			if c, ok := event.Object.(*infrastructure.KubernetesCluster); ok {
				return cmd.isAvailable(c), nil
			}
			return false, nil
		},
	},
	); err != nil {
		return err
	}

	clustercmd := auth.ClusterCmd{Name: config.ContextName(cluster), ExecPlugin: true}
	return clustercmd.Run(ctx, client)
}

func (cmd *vclusterCmd) isAvailable(cluster *infrastructure.KubernetesCluster) bool {
	return isAvailable(cluster) && len(cluster.Status.AtProvider.APIEndpoint) != 0
}

func (cmd *vclusterCmd) newCluster(project string) *infrastructure.KubernetesCluster {
	name := getName(cmd.Name)
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
					Version: cmd.KubernetesVersion,
				},
				Location: cmd.Location,
				NodePools: []infrastructure.NodePool{
					{
						Name:        cmd.NodePoolName,
						MinNodes:    cmd.MinNodes,
						MaxNodes:    cmd.MaxNodes,
						MachineType: infrastructure.NewMachineType(cmd.MachineType),
					},
				},
			},
		},
	}
}
