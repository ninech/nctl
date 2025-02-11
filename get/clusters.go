package get

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"text/tabwriter"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/config"
	"github.com/ninech/nctl/internal/format"
)

type clustersCmd struct {
	resourceCmd
}

func (l *clustersCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	clusterList := &infrastructure.KubernetesClusterList{}

	if err := get.list(ctx, client, clusterList, api.MatchName(l.Name)); err != nil {
		return err
	}

	if len(clusterList.Items) == 0 {
		get.printEmptyMessage(os.Stdout, infrastructure.KubernetesClusterKind, client.Project)
		return nil
	}

	switch get.Output {
	case full:
		return printClusters(clusterList.Items, get, true)
	case noHeader:
		return printClusters(clusterList.Items, get, false)
	case yamlOut:
		return format.PrettyPrintObjects(clusterList.GetItems(), format.PrintOpts{})
	case contexts:
		for _, cluster := range clusterList.Items {
			fmt.Printf("%s\n", config.ContextName(&cluster))
		}
	}

	return nil
}

func printClusters(clusters []infrastructure.KubernetesCluster, get *Cmd, header bool) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)

	if header {
		get.writeHeader(w, "NAME", "PROVIDER", "NUM_NODES")
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
		get.writeTabRow(w, cluster.Namespace, cluster.Name, provider, strconv.Itoa(numNodes))
	}

	return w.Flush()
}
