package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestConfigParsing(t *testing.T) {
	contextName := "test"
	cfg := NewConfig("evilcorp")
	objectCfg, err := cfg.ToObject()
	require.NoError(t, err)
	for name, testCase := range map[string]struct {
		kubeconfig    clientcmdapi.Config
		errorExpected bool
	}{
		"happy-path": {
			kubeconfig: func() clientcmdapi.Config {
				kubeCfg := testKubeconfig(contextName)
				kubeCfg.Contexts[contextName].Extensions = map[string]runtime.Object{
					NctlExtensionName: objectCfg,
				}
				return kubeCfg
			}(),
		},
		"no-config-via-extension-set": {
			kubeconfig: func() clientcmdapi.Config {
				return testKubeconfig(contextName)
			}(),
			errorExpected: true,
		},
		"no-context-found": {
			kubeconfig: func() clientcmdapi.Config {
				kubeCfg := testKubeconfig(contextName)
				delete(kubeCfg.Contexts, contextName)
				return kubeCfg
			}(),
			errorExpected: true,
		},
	} {

		testCase := testCase
		t.Run(name, func(t *testing.T) {
			content, err := clientcmd.Write(testCase.kubeconfig)
			require.NoError(t, err)

			parsedCfg, err := readConfig(content, contextName)
			if testCase.errorExpected {
				require.Error(t, err)
				return
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, parsedCfg.Organization, cfg.Organization)
		})
	}
}

func testKubeconfig(contextName string) clientcmdapi.Config {
	return clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			contextName: {
				Server: "not.so.important",
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			contextName: {
				Token: "blablubb",
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			contextName: {
				Cluster:   contextName,
				AuthInfo:  contextName,
				Namespace: "whatever",
			},
		},
		CurrentContext: contextName,
	}
}
