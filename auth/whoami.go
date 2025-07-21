package auth

import (
	"context"
	"fmt"

	"github.com/ninech/nctl/api"
)

type WhoAmICmd struct {
	APIURL    string `help:"The URL of the Nine API" default:"https://nineapis.ch" env:"NCTL_API_URL" name:"api-url"`
	IssuerURL string `help:"Issuer URL is the OIDC issuer URL of the API." default:"https://auth.nine.ch/auth/realms/pub"`
	ClientID  string `help:"Client ID is the OIDC client ID of the API." default:"nineapis.ch-f178254"`
}

func (s *WhoAmICmd) Run(ctx context.Context, client *api.Client) error {
	org, err := client.Organization()
	if err != nil {
		return err
	}

	userInfo, err := api.GetUserInfoFromToken(client.Token(ctx))
	if err != nil {
		return err
	}

	printUserInfo(userInfo, org)

	return nil
}

func printUserInfo(userInfo *api.UserInfo, org string) {
	fmt.Printf("You are currently logged in with the following account: %q\n", userInfo.User)
	fmt.Printf("Your current organization: %q\n", org)

	if len(userInfo.Orgs) > 0 {
		printAvailableOrgsString(org, userInfo.Orgs)
	}
}

func printAvailableOrgsString(currentorg string, orgs []string) {
	fmt.Println("\nAvailable Organizations:")

	for _, org := range orgs {
		activeMarker := ""
		if currentorg == org {
			activeMarker = "*"
		}

		fmt.Printf("%s\t%s\n", activeMarker, org)
	}

	fmt.Print("\nTo switch the organization use the following command:\n")
	fmt.Print("$ nctl auth set-org <org-name>\n")
}
