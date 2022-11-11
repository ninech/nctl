package auth

import (
	"fmt"

	"github.com/ninech/nctl/api"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func mergeConfig(from, to *clientcmdapi.Config) {
	for k, v := range from.Clusters {
		to.Clusters[k] = v
	}

	for k, v := range from.AuthInfos {
		to.AuthInfos[k] = v
	}

	for k, v := range from.Contexts {
		to.Contexts[k] = v
	}

	to.CurrentContext = from.CurrentContext
}

func RemoveClusterFromConfig(client *api.Client, clusterContext string) error {
	kubeconfig, err := clientcmd.LoadFromFile(client.KubeconfigPath)
	if err != nil {
		return fmt.Errorf("kubeconfig not found: %w", err)
	}

	if _, ok := kubeconfig.Clusters[clusterContext]; !ok {
		return fmt.Errorf("could not find cluster %q in kubeconfig", clusterContext)
	}

	delete(kubeconfig.Clusters, clusterContext)
	delete(kubeconfig.AuthInfos, clusterContext)
	delete(kubeconfig.Contexts, clusterContext)

	kubeconfig.CurrentContext = ""

	return clientcmd.WriteToFile(*kubeconfig, client.KubeconfigPath)
}
