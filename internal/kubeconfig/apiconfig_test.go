package kubeconfig

import (
	"net/url"
	"testing"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/config"
	"github.com/ninech/nctl/internal/cli"
	"github.com/stretchr/testify/require"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestNewAPIConfig(t *testing.T) {
	t.Parallel()

	apiURL, err := url.Parse("https://api.example.org")
	require.NoError(t, err)
	issuerURL, err := url.Parse("https://auth.example.org")
	require.NoError(t, err)
	const command = "/usr/local/bin/nctl"

	tests := []struct {
		name         string
		opts         []APIConfigOption
		wantName     string
		wantCACert   []byte
		wantOrg      string
		wantAuthInfo clientcmdapi.AuthInfo
	}{
		{
			name:     "oidc exec plugin by default",
			wantName: apiURL.Host,
			wantAuthInfo: clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					APIVersion: "client.authentication.k8s.io/v1beta1",
					Command:    command,
					Args: []string{
						api.AuthCmdName,
						api.OIDCCmdName,
						api.IssuerURLArg + issuerURL.String(),
						api.ClientIDArg + "client",
						api.UsePKCEArg,
					},
				},
			},
		},
		{
			name:         "static token",
			opts:         []APIConfigOption{UseStaticToken("token"), WithOrganization("org")},
			wantName:     apiURL.Host,
			wantOrg:      "org",
			wantAuthInfo: clientcmdapi.AuthInfo{Token: "token"},
		},
		{
			name: "client credentials exec plugin",
			opts: []APIConfigOption{UseClientCredentials(ClientCredentials{
				ClientID: "id", ClientSecret: "secret", TokenURL: "https://token.example.org",
			})},
			wantName: apiURL.Host,
			wantAuthInfo: clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					APIVersion:      "client.authentication.k8s.io/v1",
					InteractiveMode: clientcmdapi.NeverExecInteractiveMode,
					Command:         command,
					Args: []string{
						api.AuthCmdName,
						api.ClientCredentialsCmdName,
						api.ClientIDArg + "id",
						api.ClientSecretArg + "secret",
						api.TokenURLArg + "https://token.example.org",
					},
				},
			},
		},
		{
			name:       "cluster entry with overridden name and CA",
			opts:       []APIConfigOption{OverrideName("cluster/project"), WithCACert([]byte("ca"))},
			wantName:   "cluster/project",
			wantCACert: []byte("ca"),
			wantAuthInfo: clientcmdapi.AuthInfo{
				Exec: &clientcmdapi.ExecConfig{
					APIVersion: "client.authentication.k8s.io/v1beta1",
					Command:    command,
					Args: []string{
						api.AuthCmdName,
						api.OIDCCmdName,
						api.IssuerURLArg + issuerURL.String(),
						api.ClientIDArg + "client",
						api.UsePKCEArg,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			cfg, err := NewAPIConfig(apiURL, issuerURL, command, "client", tt.opts...)
			is.NoError(err)

			is.Equal(tt.wantName, cfg.CurrentContext)
			is.Len(cfg.Clusters, 1)
			is.Len(cfg.Contexts, 1)
			is.Len(cfg.AuthInfos, 1)
			is.Equal(apiURL.String(), cfg.Clusters[tt.wantName].Server)
			is.Equal(tt.wantCACert, cfg.Clusters[tt.wantName].CertificateAuthorityData)
			is.Equal(tt.wantName, cfg.Contexts[tt.wantName].Cluster)
			is.Equal(tt.wantName, cfg.Contexts[tt.wantName].AuthInfo)
			is.Equal(&tt.wantAuthInfo, cfg.AuthInfos[tt.wantName])

			wantExtension, err := config.NewExtension(tt.wantOrg).ToObject()
			is.NoError(err)
			is.Equal(wantExtension, cfg.Contexts[tt.wantName].Extensions[cli.Name])
		})
	}
}
