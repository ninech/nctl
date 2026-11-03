package kubeconfig

import (
	"fmt"
	"io/fs"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// RemoveContext removes the context name and its user from the kubeconfig of configAccess.
// The cluster of the context is removed too unless another context uses it.
// Each entry is removed from the file that defines it.
// It returns [fs.ErrNotExist] if the kubeconfig has no context name.
func RemoveContext(configAccess clientcmd.ConfigAccess, name string) error {
	config, err := configAccess.GetStartingConfig()
	if err != nil {
		return err
	}

	kubeContext, ok := config.Contexts[name]
	if !ok {
		return fmt.Errorf("kubeconfig context %q: %w", name, fs.ErrNotExist)
	}

	delete(config.Contexts, name)
	delete(config.AuthInfos, kubeContext.AuthInfo)
	if !clusterInUse(config.Contexts, kubeContext.Cluster) {
		delete(config.Clusters, kubeContext.Cluster)
	}
	if config.CurrentContext == name {
		config.CurrentContext = ""
	}

	return clientcmd.ModifyConfig(configAccess, *config, false)
}

// clusterInUse reports whether one of contexts uses cluster.
func clusterInUse(contexts map[string]*clientcmdapi.Context, cluster string) bool {
	for _, c := range contexts {
		if c.Cluster == cluster {
			return true
		}
	}
	return false
}
