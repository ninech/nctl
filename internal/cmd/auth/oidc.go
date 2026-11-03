package auth

import (
	"context"
	"os"

	"github.com/ninech/nctl/api"
)

type OIDCCmd struct {
	IssuerURL       string
	ClientID        string
	UsePKCE         bool
	InsecureStorage bool `help:"Cache the tokens in plain text files instead of the keyring of the operating system."`
}

const OIDCCmdName = api.OIDCCmdName

// Run prints the token of the OIDC exec plugin with the tokens cached in creds.
func (o *OIDCCmd) Run(ctx context.Context, creds api.Credentials) error {
	login := api.OIDCLogin{
		IssuerURL: o.IssuerURL,
		ClientID:  o.ClientID,
		UsePKCE:   o.UsePKCE,
		Storage:   storage(o.InsecureStorage),
	}

	return api.GetToken(ctx, login, creds, os.Stdout)
}
