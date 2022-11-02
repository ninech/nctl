package get

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/auth"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type clustersCmd struct{}

func (l *clustersCmd) Run(client *api.Client, get *Cmd) error {
	clusterList := &infrastructure.KubernetesClusterList{}

	listOpts := []runtimeclient.ListOption{}
	if !get.AllNamespaces {
		listOpts = append(listOpts, runtimeclient.InNamespace(client.Namespace))
	}

	if err := client.List(context.Background(), clusterList, listOpts...); err != nil {
		return err
	}

	if len(clusterList.Items) == 0 {
		if client.Namespace == "" {
			fmt.Println("no clusters found")
			return nil
		}

		fmt.Printf("no clusters found in namespace %s\n", client.Namespace)
		return nil
	}

	switch get.Output {
	case full:
		return printClusters(clusterList.Items, true)
	case noHeader:
		return printClusters(clusterList.Items, false)
	case contexts:
		for _, cluster := range clusterList.Items {
			fmt.Printf("%s\n", auth.ContextName(&cluster))
		}
	}

	return nil
}

func printClusters(clusters []infrastructure.KubernetesCluster, header bool) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)

	if header {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", "NAME", "NAMESPACE", "PROVIDER", "NUM_NODES")
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

		fmt.Fprintf(w, "%s\t%s\t%s\t%v\n", cluster.Name, cluster.Namespace, provider, numNodes)
	}

	return w.Flush()
}
