package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestConfigParsing(t *testing.T) {
	cfg := newConfig("evilcorp")
	objectCfg, err := cfg.toObject()
	require.NoError(t, err)
	kubeconfig := clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			"test": {
				Server: "not.so.important",
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"test": {
				Token: "blablubb",
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"test": {
				Cluster:   "test",
				AuthInfo:  "test",
				Namespace: "whatever",
				Extensions: map[string]runtime.Object{
					nctlExtensionName: objectCfg,
				},
			},
		},
		CurrentContext: "test",
	}

	content, err := clientcmd.Write(kubeconfig)
	require.NoError(t, err)

	parsedCfg, err := readConfig(content, "test")
	require.NoError(t, err)
	require.Equal(t, parsedCfg.Organization, cfg.Organization)
}
