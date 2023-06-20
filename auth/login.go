package auth

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/format"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type LoginCmd struct {
	Organization string `arg:"" help:"Name of the organization to login to."`
	APIURL       string `help:"The URL of the Nine API" default:"https://nineapis.ch" env:"NCTL_API_URL" name:"api-url"`
	APIToken     string `help:"Use a static API token instead of using an OIDC login." env:"NCTL_API_TOKEN"`
	IssuerURL    string `help:"Issuer URL is the OIDC issuer URL of the API." default:"https://auth.nine.ch/auth/realms/pub"`
	ClientID     string `help:"Client ID is the OIDC client ID of the API." default:"nineapis.ch-f178254"`
	ExecPlugin   bool   `help:"Automatically run exec plugin after writing the kubeconfig." hidden:"" default:"true"`
}

const (
	LoginCmdName = "auth login"
)

func (l *LoginCmd) Run(ctx context.Context, command string) error {
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

	opts := []apiConfigOption{withOrganization(l.Organization)}
	if len(l.APIToken) != 0 {
		l.ExecPlugin = false
		opts = append(opts, useStaticToken(l.APIToken))
	}

	cfg, err := newAPIConfig(apiURL, issuerURL, command, l.ClientID, opts...)
	if err != nil {
		return err
	}

	return login(ctx, cfg, loadingRules.GetDefaultFilename(), runExecPlugin(l.ExecPlugin), project(l.Organization))
}

type apiConfig struct {
	name         string
	token        string
	caCert       []byte
	organization string
}

type apiConfigOption func(*apiConfig)

func overrideName(name string) apiConfigOption {
	return func(ac *apiConfig) {
		ac.name = name
	}
}

func setCACert(caCert []byte) apiConfigOption {
	return func(ac *apiConfig) {
		ac.caCert = caCert
	}
}

func useStaticToken(token string) apiConfigOption {
	return func(ac *apiConfig) {
		ac.token = token
	}
}

func withOrganization(organization string) apiConfigOption {
	return func(ac *apiConfig) {
		ac.organization = organization
	}
}

func newAPIConfig(apiURL, issuerURL *url.URL, command, clientID string, opts ...apiConfigOption) (*clientcmdapi.Config, error) {
	cfg := &apiConfig{
		name: apiURL.Host,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	extension, err := NewConfig(cfg.organization).ToObject()
	if err != nil {
		return nil, err
	}

	clientConfig := &clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			cfg.name: {
				Server:                   apiURL.String(),
				CertificateAuthorityData: cfg.caCert,
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			cfg.name: {
				Cluster:  cfg.name,
				AuthInfo: cfg.name,
				Extensions: map[string]runtime.Object{
					util.NctlName: extension,
				},
			},
		},
		AuthInfos:      map[string]*clientcmdapi.AuthInfo{},
		CurrentContext: cfg.name,
	}

	if len(cfg.token) != 0 {
		clientConfig.AuthInfos[cfg.name] = &clientcmdapi.AuthInfo{
			Token: cfg.token,
		}
		return clientConfig, nil
	}

	// we make sure our command is in the $PATH as the client-go credential plugin will need to find it.
	if _, err := exec.LookPath(command); err != nil && command != "" {
		return nil, fmt.Errorf("%s not found in $PATH, please add it first before logging in", command)
	}

	clientConfig.AuthInfos[cfg.name] = &clientcmdapi.AuthInfo{
		Exec: execConfig(command, clientID, issuerURL),
	}

	return clientConfig, nil
}

type loginConfig struct {
	execPlugin           bool
	project              string
	switchCurrentContext bool
}

type loginOption func(*loginConfig)

// runExecPlugin runs the exec plugin after building the new config
func runExecPlugin(enabled bool) loginOption {
	return func(l *loginConfig) {
		l.execPlugin = enabled
	}
}

// project overrides the project in the new config
func project(project string) loginOption {
	return func(l *loginConfig) {
		l.project = project
	}
}

// switchCurrentContext sets the context of the merged kubeconfig to the one
// defined in the newConfig
func switchCurrentContext() loginOption {
	return func(l *loginConfig) {
		l.switchCurrentContext = true
	}
}

func login(ctx context.Context, newConfig *clientcmdapi.Config, kubeconfigPath string, opts ...loginOption) error {
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

	format.PrintSuccessf("📋", "added %s to kubeconfig", newConfig.CurrentContext)
	if loginConfig.execPlugin {
		authInfo := newConfig.AuthInfos[newConfig.CurrentContext]
		if authInfo == nil || authInfo.Exec == nil {
			return fmt.Errorf("no Exec found in authInfo")
		}

		// we discard the returned token as we just want to trigger the auth
		// flow and populate the cache.
		if _, err := api.GetTokenFromExecConfig(ctx, authInfo.Exec); err != nil {
			return err
		}

		format.PrintSuccessf("🚀", "logged into cluster %s", newConfig.CurrentContext)
	}

	return nil
}
