package auth

import (
	"context"
	"os"

	"github.com/ninech/nctl/api"
)

type OIDCCmd struct {
	IssuerURL string
	ClientID  string
	UsePKCE   bool
}

const OIDCCmdName = api.OIDCCmdName

func (o *OIDCCmd) Run(ctx context.Context) error {
	return api.GetToken(ctx, o.IssuerURL, o.ClientID, o.UsePKCE, os.Stdout)
}
