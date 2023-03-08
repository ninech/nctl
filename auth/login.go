package auth

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"

	"github.com/ninech/nctl/api"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type LoginCmd struct {
	Organization string `arg:"" help:"Name of the organization to login to."`
	APIURL       string `help:"The URL of the Nine API" default:"https://nineapis.ch"`
	IssuerURL    string `help:"Issuer URL is the OIDC issuer URL of the API." default:"https://auth.nine.ch/auth/realms/pub"`
	ClientID     string `help:"Client ID is the OIDC client ID of the API." default:"nineapis.ch-f178254"`
	ExecPlugin   bool   `help:"Automatically run exec plugin after writing the kubeconfig." hidden:"" default:"true"`
}

const (
	LoginCmdName = "auth login"
)

func (l *LoginCmd) Run(command string) error {
	loadingRules, err := api.LoadingRules()
	if err != nil {
		return err
	}

	apiURL, err := url.Parse(l.APIURL)
	if err != nil {
		return err
	}

	issuerURL, err := url.Parse(l.IssuerURL)
	if err != nil {
		return err
	}

	cfg, err := apiConfig(apiURL, issuerURL, command, l.ClientID)
	if err != nil {
		return err
	}

	return login(cfg, loadingRules.GetDefaultFilename(), runExecPlugin(l.ExecPlugin), namespace(l.Organization))
}

func apiConfig(apiURL, issuerURL *url.URL, command, clientID string) (*clientcmdapi.Config, error) {
	if _, err := exec.LookPath(command); err != nil && command != "" {
		return nil, fmt.Errorf("%s not found in $PATH, please add it first before logging in", command)
	}

	return &clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			apiURL.Host: {
				Server: apiURL.String(),
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			apiURL.Host: {
				Cluster:  apiURL.Host,
				AuthInfo: apiURL.Host,
			},
		},
		CurrentContext: apiURL.Host,
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			apiURL.Host: {
				Exec: execConfig(command, clientID, issuerURL),
			},
		},
	}, nil
}

type loginConfig struct {
	execPlugin bool
	namespace  string
}

type loginOption func(*loginConfig)

// runExecPlugin runs the exec plugin after building the new config
func runExecPlugin(enabled bool) loginOption {
	return func(l *loginConfig) {
		l.execPlugin = enabled
	}
}

// namespace overrides the namespace in the new config
func namespace(context string) loginOption {
	return func(l *loginConfig) {
		l.namespace = context
	}
}

func login(newConfig *clientcmdapi.Config, kubeconfigPath string, opts ...loginOption) error {
	loginConfig := &loginConfig{}
	for _, opt := range opts {
		opt(loginConfig)
	}

	if loginConfig.namespace != "" && newConfig.Contexts[newConfig.CurrentContext] != nil {
		newConfig.Contexts[newConfig.CurrentContext].Namespace = loginConfig.namespace
	}

	kubeconfig, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// kubeconfig file does not exist so we just use our new config
		kubeconfig = newConfig
	}

	mergeConfig(newConfig, kubeconfig)

	if err := clientcmd.WriteToFile(*kubeconfig, kubeconfigPath); err != nil {
		return err
	}

	fmt.Printf(" ✓ added %s to kubeconfig 📋\n", kubeconfig.CurrentContext)

	if loginConfig.execPlugin {
		authInfo := newConfig.AuthInfos[newConfig.CurrentContext]
		if authInfo == nil || authInfo.Exec == nil {
			return fmt.Errorf("no Exec found in authInfo")
		}

		cmd := exec.Command(authInfo.Exec.Command, authInfo.Exec.Args...)
		// we want to see potential errors of the auth plugin
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("unable to login, make sure kubectl and kubelogin is installed: %w", err)
		}

		fmt.Printf(" ✓ logged into cluster %s 🚀\n", kubeconfig.CurrentContext)
	}

	return err
}
