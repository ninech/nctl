// Package kubeconfig builds the kubeconfig entries nctl uses to talk to the Nine API and to Kubernetes clusters,
// and merges them into the user's kubeconfig.
package kubeconfig

import (
	"net/url"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/config"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

// ClientCredentials identify the APIServiceAccount the exec plugin obtains a token for.
// The client secret is not part of the kubeconfig, the exec plugin reads it with [api.LookupClientSecret].
type ClientCredentials struct {
	ClientID string
	TokenURL string
}

type apiConfig struct {
	name         string
	token        string
	credentials  ClientCredentials
	caCert       []byte
	organization string
	storage      api.Storage
}

// APIConfigOption configures the kubeconfig built by [NewAPIConfig].
type APIConfigOption func(*apiConfig)

// OverrideName sets the name used for the cluster,
// context and user entry instead of the API URL host.
func OverrideName(name string) APIConfigOption {
	return func(ac *apiConfig) {
		ac.name = name
	}
}

// WithCACert sets the certificate authority data of the cluster entry.
func WithCACert(caCert []byte) APIConfigOption {
	return func(ac *apiConfig) {
		ac.caCert = caCert
	}
}

// UseStaticToken authenticates with a static token instead of an exec plugin.
func UseStaticToken(token string) APIConfigOption {
	return func(ac *apiConfig) {
		ac.token = token
	}
}

// UseClientCredentials authenticates through the client-credentials exec plugin instead of the interactive OIDC one.
func UseClientCredentials(credentials ClientCredentials) APIConfigOption {
	return func(ac *apiConfig) {
		ac.credentials = credentials
	}
}

// WithStorage sets where the exec plugin caches its tokens or reads its client secret,
// [api.StorageKeyring] by default.
func WithStorage(storage api.Storage) APIConfigOption {
	return func(ac *apiConfig) {
		ac.storage = storage
	}
}

// WithOrganization records the organization in the nctl context extension.
func WithOrganization(organization string) APIConfigOption {
	return func(ac *apiConfig) {
		ac.organization = organization
	}
}

// NewAPIConfig returns a kubeconfig with a single cluster,
// context and user for the API at apiURL.
// Unless a static token or client credentials are configured,
// the user authenticates through the OIDC exec plugin of the nctl binary at command.
func NewAPIConfig(apiURL, issuerURL *url.URL, command, clientID string, opts ...APIConfigOption) (*clientcmdapi.Config, error) {
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
					api.Name: extension,
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

	if cfg.credentials.ClientID != "" {
		clientConfig.AuthInfos[cfg.name] = &clientcmdapi.AuthInfo{
			Exec: clientCredentialsExecConfig(command, cfg.credentials, cfg.storage),
		}
		return clientConfig, nil
	}

	clientConfig.AuthInfos[cfg.name] = &clientcmdapi.AuthInfo{
		Exec: oidcExecConfig(command, clientID, issuerURL, cfg.storage),
	}

	return clientConfig, nil
}

// oidcExecConfig returns an exec config that obtains a token through the interactive OIDC login of nctl.
func oidcExecConfig(command, clientID string, issuerURL *url.URL, storage api.Storage) *clientcmdapi.ExecConfig {
	args := api.ExecArgs{
		Command:   api.OIDCCmdName,
		IssuerURL: issuerURL.String(),
		ClientID:  clientID,
		UsePKCE:   true,
		Storage:   storage,
	}

	return &clientcmdapi.ExecConfig{
		APIVersion: "client.authentication.k8s.io/v1beta1",
		Command:    command,
		Args:       args.Args(),
	}
}

// clientCredentialsExecConfig returns an exec config that obtains a token through the client-credentials login of nctl.
func clientCredentialsExecConfig(command string, credentials ClientCredentials, storage api.Storage) *clientcmdapi.ExecConfig {
	return &clientcmdapi.ExecConfig{
		APIVersion:      "client.authentication.k8s.io/v1",
		InteractiveMode: clientcmdapi.NeverExecInteractiveMode,
		Command:         command,
		Args: api.ExecArgs{
			Command:  api.ClientCredentialsCmdName,
			ClientID: credentials.ClientID,
			TokenURL: credentials.TokenURL,
			Storage:  storage,
		}.Args(),
	}
}
