package auth

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"strings"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/config"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/format"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type LoginCmd struct {
	APIURL                      string `help:"The URL of the Nine API" default:"https://nineapis.ch" env:"NCTL_API_URL" name:"api-url"`
	APIToken                    string `help:"Use a static API token instead of using an OIDC login. You need to specify the --organization parameter as well." env:"NCTL_API_TOKEN"`
	Organization                string `help:"The name of your organization to use when providing an API token. This parameter is only used when providing a API token. This parameter needs to be set if you use --api-token." env:"NCTL_ORGANIZATION"`
	IssuerURL                   string `help:"Issuer URL is the OIDC issuer URL of the API." default:"https://auth.nine.ch/auth/realms/pub"`
	ClientID                    string `help:"Client ID is the OIDC client ID of the API." default:"nineapis.ch-f178254"`
	ForceInteractiveEnvOverride bool   `help:"Used for internal purposes only. Set to true to force interactive environment explicit override. Set to false to fall back to automatic interactivity detection." default:"false" hidden:""`
	tk                          api.TokenGetter
}

const ErrNonInteractiveEnvironmentEmptyToken = "a static API token is required in non-interactive environments"

func (l *LoginCmd) Run(ctx context.Context) error {
	apiURL, err := url.Parse(l.APIURL)
	if err != nil {
		return err
	}

	issuerURL, err := url.Parse(l.IssuerURL)
	if err != nil {
		return err
	}

	loadingRules, err := api.LoadingRules()
	if err != nil {
		return err
	}

	command, err := os.Executable()
	if err != nil {
		return fmt.Errorf("can not identify executable path of %s: %v", util.NctlName, err)
	}

	if len(l.APIToken) != 0 {
		if len(l.Organization) == 0 {
			return fmt.Errorf("you need to set the --organization parameter explicitly if you use --api-token")
		}

		userInfo, err := api.GetUserInfoFromToken(l.APIToken)
		if err != nil {
			return err
		}

		cfg, err := newAPIConfig(apiURL, issuerURL, command, l.ClientID, useStaticToken(l.APIToken), withOrganization(l.Organization))
		if err != nil {
			return err
		}

		return login(ctx, cfg, loadingRules.GetDefaultFilename(), userInfo.User, "", project(l.Organization))
	}

	if !l.ForceInteractiveEnvOverride && !format.IsInteractiveEnvironment(os.Stdout) {
		return errors.New(ErrNonInteractiveEnvironmentEmptyToken)
	}

	usePKCE := true

	token, err := l.tokenGetter().GetTokenString(ctx, l.IssuerURL, l.ClientID, usePKCE)
	if err != nil {
		return err
	}

	userInfo, err := api.GetUserInfoFromToken(token)
	if err != nil {
		return err
	}

	if len(userInfo.Orgs) == 0 {
		return fmt.Errorf("error getting an organization for the account %q. Please contact support", userInfo.User)
	}

	org := userInfo.Orgs[0]
	if len(userInfo.Orgs) > 1 {
		fmt.Printf("Multiple organizations found for the account %q.\n", userInfo.User)
		fmt.Printf("Defaulting to %q\n", org)
		printAvailableOrgsString(org, userInfo.Orgs)
	}

	cfg, err := newAPIConfig(apiURL, issuerURL, command, l.ClientID, withOrganization(org))
	if err != nil {
		return err
	}

	return login(ctx, cfg, loadingRules.GetDefaultFilename(), userInfo.User, "", project(org))
}

func (l *LoginCmd) tokenGetter() api.TokenGetter {
	if l.tk != nil {
		return l.tk
	}
	return &api.DefaultTokenGetter{}
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

	extension, err := config.NewExtension(cfg.organization).ToObject()
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

	clientConfig.AuthInfos[cfg.name] = &clientcmdapi.AuthInfo{
		Exec: execConfig(command, clientID, issuerURL),
	}

	return clientConfig, nil
}

type loginConfig struct {
	project              string
	switchCurrentContext bool
}

type loginOption func(*loginConfig)

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

func login(ctx context.Context, newConfig *clientcmdapi.Config, kubeconfigPath, userName string, toOrg string, opts ...loginOption) error {
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
		format.PrintSuccessf("🏢", "switched to the organization %q", toOrg)
	}
	format.PrintSuccessf("📋", "added %s to kubeconfig", newConfig.CurrentContext)

	loginMessage := fmt.Sprintf("logged into cluster %s", newConfig.CurrentContext)
	if strings.TrimSpace(userName) != "" {
		loginMessage = fmt.Sprintf("logged into cluster %s as %s", newConfig.CurrentContext, userName)
	}
	format.PrintSuccess("🚀", loginMessage)

	return nil
}

func mergeKubeConfig(from, to *clientcmdapi.Config) {
	maps.Copy(to.Clusters, from.Clusters)
	maps.Copy(to.AuthInfos, from.AuthInfos)
	maps.Copy(to.Contexts, from.Contexts)
}
