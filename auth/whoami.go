package auth

import (
	"context"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type WhoAmICmd struct {
	format.Writer `hidden:""`
	APIURL        string `help:"URL of the Nine API." default:"https://nineapis.ch" env:"NCTL_API_URL" name:"api-url"`
	IssuerURL     string `help:"OIDC issuer URL of the API." default:"https://auth.nine.ch/auth/realms/pub"`
	ClientID      string `help:"OIDC client ID of the API." default:"nineapis.ch-f178254"`
}

func (cmd *WhoAmICmd) Run(ctx context.Context, client *api.Client) error {
	org, err := client.Organization()
	if err != nil {
		return err
	}

	userInfo, err := api.GetUserInfoFromToken(client.Token(ctx))
	if err != nil {
		return err
	}

	cmd.printUserInfo(userInfo, org)

	return nil
}

func (cmd *WhoAmICmd) printUserInfo(userInfo *api.UserInfo, org string) {
	cmd.Infof("👤", "You are currently logged in with the following account: %q", userInfo.User)
	cmd.Infof("🏢", "Your current organization: %q", org)

	if len(userInfo.Orgs) > 0 {
		printAvailableOrgsString(cmd.Writer, org, userInfo.Orgs)
	}
}
