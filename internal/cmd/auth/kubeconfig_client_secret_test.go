package auth

import (
	"testing"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/cli"
	"github.com/stretchr/testify/assert"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestKubeconfigClientSecretLogin(t *testing.T) {
	t.Parallel()

	legacy := api.ExecArgs{Command: api.ClientCredentialsCmdName, ClientID: "id", ClientSecret: "secret", TokenURL: defaultTokenURL}
	exec := func(args api.ExecArgs) *clientcmdapi.ExecConfig {
		return &clientcmdapi.ExecConfig{Args: args.Args()}
	}
	withTokenURL := legacy
	withTokenURL.TokenURL = "https://token.example.org"
	stored := legacy
	stored.ClientSecret = ""

	tests := []struct {
		name         string
		exec         *clientcmdapi.ExecConfig
		apiURL       string
		organization string
		want         string
		wantOK       bool
	}{
		{
			name:   "leaves out defaults",
			exec:   exec(legacy),
			apiURL: defaultAPIURL + "/",
			want:   cli.LoginCmd + " --api-client-id=id --api-client-secret=<secret>",
			wantOK: true,
		},
		{
			name:         "keeps the organization, API and token URL",
			exec:         exec(withTokenURL),
			apiURL:       "https://api.example.org",
			organization: "org",
			want:         cli.LoginCmd + " --api-client-id=id --api-client-secret=<secret> --organization=org --api-url=https://api.example.org --api-token-url=https://token.example.org",
			wantOK:       true,
		},
		{name: "stored client secret", exec: exec(stored)},
		{name: "OIDC login", exec: exec(api.ExecArgs{Command: api.OIDCCmdName, IssuerURL: "https://issuer.example.org", ClientID: "id"})},
		{name: "foreign exec plugin", exec: &clientcmdapi.ExecConfig{Args: []string{"get-token"}}},
		{name: "no exec plugin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := KubeconfigClientSecretLogin(tt.exec, tt.apiURL, tt.organization)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.want, got)
			assert.NotContains(t, got, "=secret")
		})
	}
}
