package auth

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/url"
	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/config"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/format"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	defaultClientID  = "nineapis.ch-f178254"
	defaultIssuerURL = "https://auth.nine.ch/auth/realms/pub"
	defaultTokenURL  = defaultIssuerURL + "/protocol/openid-connect/token"
)

type LoginCmd struct {
	format.Writer
	API                         API    `embed:"" prefix:"api-"`
	Organization                string `help:"Name of your organization to use when providing an API client ID/secret." env:"NCTL_ORGANIZATION"`
	IssuerURL                   string `help:"OIDC issuer URL of the API." default:"${issuer_url}" hidden:""`
	ClientID                    string `help:"OIDC client ID of the API." default:"${client_id}" hidden:""`
	ForceInteractiveEnvOverride bool   `help:"Used for internal purposes only. Set to true to force interactive environment explicit override. Set to false to fall back to automatic interactivity detection." default:"false" hidden:""`
	tk                          api.TokenGetter
}

const ErrNonInteractiveEnvironmentEmptyToken = "a static API token is required in non-interactive environments"

func (l *LoginCmd) Run(ctx context.Context) error {
	apiURL, err := url.Parse(l.API.URL)
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

	if l.API.Token != "" {
		if l.Organization == "" {
			return fmt.Errorf("you need to set the --organization parameter explicitly if you use --api-token")
		}
		userInfo, err := api.GetUserInfoFromToken(l.API.Token)
		if err != nil {
			return err
		}
		cfg, err := newAPIConfig(apiURL, issuerURL, command, l.ClientID, useStaticToken(l.API.Token), withOrganization(l.Organization))
		if err != nil {
			return err
		}
		return login(l.Writer, cfg, loadingRules.GetDefaultFilename(), userInfo.User, "", project(l.Organization))
	}

	if l.API.ClientID != "" {
		userInfo, err := l.API.UserInfo(ctx)
		if err != nil {
			return err
		}
		if l.Organization == "" && len(userInfo.Orgs) == 0 {
			return fmt.Errorf("unable to find organization, you need to set the --organization parameter explicitly")
		}
		org := l.Organization
		if org == "" {
			org = userInfo.Orgs[0]
		}
		cfg, err := newAPIConfig(apiURL, issuerURL, command, l.API.ClientID, useClientCredentials(l.API), withOrganization(org))
		if err != nil {
			return err
		}
		proj := org
		if userInfo.Project != "" {
			proj = userInfo.Project
		}
		return login(l.Writer, cfg, loadingRules.GetDefaultFilename(), userInfo.User, "", project(proj))
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
		l.Printf("Multiple organizations found for the account %q.\n", userInfo.User)
		l.Printf("Defaulting to %q\n", org)
		l.printAvailableOrgsString(org, userInfo.Orgs)
	}

	cfg, err := newAPIConfig(apiURL, issuerURL, command, l.ClientID, withOrganization(org))
	if err != nil {
		return err
	}

	return login(l.Writer, cfg, loadingRules.GetDefaultFilename(), userInfo.User, "", project(org))
}

func (l *LoginCmd) printAvailableOrgsString(currentorg string, orgs []string) {
	l.Println("\nAvailable Organizations:")

	for _, org := range orgs {
		activeMarker := ""
		if currentorg == org {
			activeMarker = "*"
		}

		l.Printf("%s\t%s\n", activeMarker, org)
	}

	l.Printf("\nTo switch the organization use the following command:\n")
	l.Printf("$ nctl auth set-org <org-name>\n")
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
	api          API
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

func useClientCredentials(api API) apiConfigOption {
	return func(ac *apiConfig) {
		ac.api = api
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

	if cfg.api.ClientID != "" {
		clientConfig.AuthInfos[cfg.name] = &clientcmdapi.AuthInfo{
			Exec: apiExecConfig(command, cfg.api),
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

func login(w format.Writer, newConfig *clientcmdapi.Config, kubeconfigPath, userName string, toOrg string, opts ...loginOption) error {
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

// LoginKongVars returns all variables which are used in the login command
func LoginKongVars() kong.Vars {
	result := make(kong.Vars)
	result["client_id"] = defaultClientID
	result["issuer_url"] = defaultIssuerURL
	result["token_url"] = defaultTokenURL
	return result
}
