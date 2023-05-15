package auth

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"

	"github.com/alecthomas/kong"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
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

	cfg, err := newAPIConfig(apiURL, issuerURL, command, l.ClientID)
	if err != nil {
		return err
	}

	return login(cfg, loadingRules.GetDefaultFilename(), runExecPlugin(l.ExecPlugin), namespace(l.Organization))
}

type apiConfig struct {
	name   string
	caCert []byte
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

func newAPIConfig(apiURL, issuerURL *url.URL, command, clientID string, opts ...apiConfigOption) (*clientcmdapi.Config, error) {
	cfg := &apiConfig{
		name: apiURL.Host,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	// we make sure our command is in the $PATH as the client-go credential plugin will need to find it.
	if _, err := exec.LookPath(command); err != nil && command != "" {
		return nil, fmt.Errorf("%s not found in $PATH, please add it first before logging in", command)
	}

	return &clientcmdapi.Config{
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
			},
		},
		CurrentContext: cfg.name,
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			cfg.name: {
				Exec: execConfig(command, clientID, issuerURL),
			},
		},
	}, nil
}

type loginConfig struct {
	execPlugin           bool
	namespace            string
	switchCurrentContext bool
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

// switchCurrentContext sets the context of the merged kubeconfig to the one
// defined in the newConfig
func switchCurrentContext() loginOption {
	return func(l *loginConfig) {
		l.switchCurrentContext = true
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

		// construct and run the auth oidc command via kong
		parser, err := kong.New(&OIDCCmd{})
		if err != nil {
			return fmt.Errorf("unable to parse OIDCCmd: %w", err)
		}
		ctx, err := parser.Parse(authInfo.Exec.Args[2:])
		if err != nil {
			return fmt.Errorf("unable to parse args: %w", err)
		}
		ctx.BindTo(context.Background(), (*context.Context)(nil))
		// we want to discard the output of the login command as we don't need
		// the returned token in this case.
		ctx.BindTo(io.Discard, (*io.Writer)(nil))

		if err := ctx.Run(); err != nil {
			return fmt.Errorf("unable to login: %w", err)
		}

		format.PrintSuccessf("🚀", "logged into cluster %s", newConfig.CurrentContext)
	}

	return err
}
