package auth

import (
	"context"
	"io"
	"net/url"

	"github.com/ninech/nctl/api"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type OIDCCmd struct {
	IssuerURL string
	ClientID  string
	UsePKCE   bool
}

const OIDCCmdName = "auth oidc"

func (o *OIDCCmd) Run(ctx context.Context, out io.Writer) error {
	return api.GetToken(ctx, o.IssuerURL, o.ClientID, o.UsePKCE, out)
}

// execConfig returns an *clientcmdapi.ExecConfig that can be used to login to
// a kubernetes cluster using nctl.
func execConfig(command, clientID string, issuerURL *url.URL) *clientcmdapi.ExecConfig {
	return &clientcmdapi.ExecConfig{
		APIVersion: "client.authentication.k8s.io/v1beta1",
		Command:    command,
		Args: []string{
			"auth",
			"oidc",
			api.IssuerURLArg + issuerURL.String(),
			api.ClientIDArg + clientID,
			api.UsePKCEArg,
		},
	}
}
