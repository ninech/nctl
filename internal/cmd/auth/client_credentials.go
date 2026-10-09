package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/kubeconfig"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauthenticationv1 "k8s.io/client-go/pkg/apis/clientauthentication/v1"
)

type API struct {
	URL          string `help:"URL of the Nine API." default:"https://nineapis.ch" env:"NCTL_API_URL" hidden:""`
	Token        string `help:"Use a static API token instead of using an OIDC login. Requires the --organization to also be set." env:"NCTL_API_TOKEN" hidden:""`
	ClientID     string `help:"Use an API client ID to login. Requires the --organization to also be set." env:"NCTL_API_CLIENT_ID"`
	ClientSecret string `help:"Use an API client secret to login. Requires the --organization to also be set." env:"NCTL_API_CLIENT_SECRET"`
	TokenURL     string `help:"Override the default client token URL." hidden:"" default:"${token_url}" env:"NCTL_API_TOKEN_URL"`
}

type ClientCredentialsCmd struct {
	API `embed:""`
}

const ClientCredentialsCmdName = api.ClientCredentialsCmdName

func (c *ClientCredentialsCmd) Run(ctx context.Context) error {
	token, err := c.Oauth2Token(ctx)
	if err != nil {
		return err
	}
	execCredential := &clientauthenticationv1.ExecCredential{
		TypeMeta: metav1.TypeMeta{
			APIVersion: clientauthenticationv1.SchemeGroupVersion.String(),
			Kind:       "ExecCredential",
		},
		Status: &clientauthenticationv1.ExecCredentialStatus{
			Token:               token.AccessToken,
			ExpirationTimestamp: new(metav1.NewTime(token.Expiry)),
		},
	}
	return json.NewEncoder(os.Stdout).Encode(execCredential)
}

func (a *API) Oauth2Token(ctx context.Context) (*oauth2.Token, error) {
	clientCredentialsCfg := &clientcredentials.Config{
		ClientID:     a.ClientID,
		ClientSecret: a.ClientSecret,
		TokenURL:     a.TokenURL,
	}
	tok, err := clientCredentialsCfg.Token(ctx)
	if rerr, ok := err.(*oauth2.RetrieveError); ok && rerr.ErrorCode == "invalid_client" {
		redactedClientSecret := a.ClientSecret
		if len(a.ClientSecret) > 3 {
			redactedClientSecret = a.ClientSecret[:3] + "<redacted>"
		}
		return nil, fmt.Errorf(
			"%s: the used client ID/secret %q/%q is invalid",
			rerr.ErrorDescription,
			a.ClientID, redactedClientSecret,
		)
	}
	return tok, err
}

func (a *API) UserInfo(ctx context.Context) (*api.UserInfo, error) {
	token, err := a.Oauth2Token(ctx)
	if err != nil {
		return nil, err
	}
	return api.GetUserInfoFromToken(token.AccessToken)
}

// clientCredentials returns the credentials the kubeconfig exec plugin uses.
func (a API) clientCredentials() kubeconfig.ClientCredentials {
	return kubeconfig.ClientCredentials{
		ClientID:     a.ClientID,
		ClientSecret: a.ClientSecret,
		TokenURL:     a.TokenURL,
	}
}
