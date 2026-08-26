package auth

import (
	"context"
	"net/url"
	"os"

	"github.com/ninech/nctl/api"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type OIDCCmd struct {
	IssuerURL     string
	ClientID      string
	UsePKCE       bool
	UseDeviceCode bool
}

const OIDCCmdName = api.OIDCCmdName

func (o *OIDCCmd) Run(ctx context.Context) error {
	return api.GetToken(ctx, o.IssuerURL, o.ClientID, o.UsePKCE, o.UseDeviceCode, os.Stdout)
}

// execConfig returns an *clientcmdapi.ExecConfig that can be used to login to
// a kubernetes cluster using nctl.
func execConfig(command, clientID string, issuerURL *url.URL, useDeviceCode bool) *clientcmdapi.ExecConfig {
	args := []string{
		CmdName,
		OIDCCmdName,
		api.IssuerURLArg + issuerURL.String(),
		api.ClientIDArg + clientID,
		api.UsePKCEArg,
	}
	if useDeviceCode {
		args = append(args, api.UseDeviceCodeArg)
	}

	return &clientcmdapi.ExecConfig{
		APIVersion: "client.authentication.k8s.io/v1beta1",
		Command:    command,
		Args:       args,
	}
}
