package get

import (
	"context"
	"strconv"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/config"
	"github.com/ninech/nctl/internal/format"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type clustersCmd struct {
	resourceCmd
}

func (cmd *clustersCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	return get.listPrint(ctx, client, cmd, api.MatchName(cmd.Name))
}

func (cmd *clustersCmd) list() client.ObjectList {
	return &infrastructure.KubernetesClusterList{}
}

func (cmd *clustersCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	clusterList := list.(*infrastructure.KubernetesClusterList)
	if len(clusterList.Items) == 0 {
		return out.printEmptyMessage(infrastructure.KubernetesClusterKind, client.Project)
	}

	switch out.Format {
	case full:
		return printClusters(clusterList.Items, out, true)
	case noHeader:
		return printClusters(clusterList.Items, out, false)
	case yamlOut:
		return format.PrettyPrintObjects(clusterList.GetItems(), format.PrintOpts{Out: &out.Writer})
	case jsonOut:
		return format.PrettyPrintObjects(
			clusterList.GetItems(),
			format.PrintOpts{
				Out:    &out.Writer,
				Format: format.OutputFormatTypeJSON,
				JSONOpts: format.JSONOutputOptions{
					PrintSingleItem: cmd.Name != "",
				},
			})
	case contexts:
		for _, cluster := range clusterList.Items {
			out.Printf("%s\n", config.ContextName(&cluster))
		}
	}

	return nil
}

func printClusters(clusters []infrastructure.KubernetesCluster, out *output, header bool) error {
	if header {
		out.writeHeader("NAME", "LOCATION", "PROVIDER", "NODES")
	}

	for _, cluster := range clusters {
		numNodes := 0
		for _, pool := range cluster.Status.AtProvider.NodePools {
			numNodes += pool.NumNodes
		}

		provider := ""
		if cluster.Spec.ForProvider.NKE != nil {
			provider = "nke"
		}

		if cluster.Spec.ForProvider.VCluster != nil {
			provider = "vcluster"
		}
		out.writeTabRow(cluster.Namespace, cluster.Name, string(cluster.Spec.ForProvider.Location), provider, strconv.Itoa(numNodes))
	}

	return out.tabWriter.Flush()
}
