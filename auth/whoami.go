package auth

import (
	"context"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type WhoAmICmd struct {
	format.Writer `kong:"-"`
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
	cmd.Printf("You are currently logged in with the following account: %q\n", userInfo.User)
	cmd.Printf("Your current organization: %q\n", org)

	if len(userInfo.Orgs) > 0 {
		cmd.printAvailableOrgsString(org, userInfo.Orgs)
	}
}

func (cmd *WhoAmICmd) printAvailableOrgsString(currentorg string, orgs []string) {
	cmd.Println("\nAvailable Organizations:")

	for _, org := range orgs {
		activeMarker := ""
		if currentorg == org {
			activeMarker = "*"
		}

		cmd.Printf("%s\t%s\n", activeMarker, org)
	}

	cmd.Printf("\nTo switch the organization use the following command:\n")
	cmd.Printf("$ nctl auth set-org <org-name>\n")
}
