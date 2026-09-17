package kubeconfig

import (
	"fmt"
	"maps"
	"os"
	"strings"

	"github.com/ninech/nctl/internal/format"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type loginConfig struct {
	project              string
	switchCurrentContext bool
}

// LoginOption configures [Login].
type LoginOption func(*loginConfig)

// WithProject sets the project (namespace) of the new config's context.
func WithProject(project string) LoginOption {
	return func(l *loginConfig) {
		l.project = project
	}
}

// SwitchCurrentContext sets the current context of the merged kubeconfig to
// the one of the new config.
func SwitchCurrentContext() LoginOption {
	return func(l *loginConfig) {
		l.switchCurrentContext = true
	}
}

// Login merges newConfig into the kubeconfig at kubeconfigPath, creating the
// file if it does not exist, and reports the result to w. userName and
// toOrg are only used in the messages and may be empty.
func Login(w format.Writer, newConfig *clientcmdapi.Config, kubeconfigPath, userName string, toOrg string, opts ...LoginOption) error {
	loginConfig := &loginConfig{}
	for _, opt := range opts {
		opt(loginConfig)
	}

	if loginConfig.project != "" && newConfig.Contexts[newConfig.CurrentContext] != nil {
		newConfig.Contexts[newConfig.CurrentContext].Namespace = loginConfig.project
	}

	kubeconfig, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// kubeconfig file does not exist so we just use our new config
		kubeconfig = newConfig
	}

	mergeKubeConfig(newConfig, kubeconfig)

	if loginConfig.switchCurrentContext {
		kubeconfig.CurrentContext = newConfig.CurrentContext
	}

	if err := clientcmd.WriteToFile(*kubeconfig, kubeconfigPath); err != nil {
		return err
	}

	if toOrg != "" {
		w.Successf("🏢", "switched to the organization %q", toOrg)
	}
	w.Successf("📋", "added %s to kubeconfig", newConfig.CurrentContext)

	loginMessage := fmt.Sprintf("logged into cluster %s", newConfig.CurrentContext)
	if strings.TrimSpace(userName) != "" {
		loginMessage = fmt.Sprintf("logged into cluster %s as %s", newConfig.CurrentContext, userName)
	}
	w.Success("🚀", loginMessage)

	return nil
}

func mergeKubeConfig(from, to *clientcmdapi.Config) {
	maps.Copy(to.Clusters, from.Clusters)
	maps.Copy(to.AuthInfos, from.AuthInfos)
	maps.Copy(to.Contexts, from.Contexts)
}
