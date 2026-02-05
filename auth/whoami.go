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

func (s *WhoAmICmd) Run(ctx context.Context, client *api.Client) error {
	org, err := client.Organization()
	if err != nil {
		return err
	}

	userInfo, err := api.GetUserInfoFromToken(client.Token(ctx))
	if err != nil {
		return err
	}

	s.printUserInfo(userInfo, org)

	return nil
}

func (s *WhoAmICmd) printUserInfo(userInfo *api.UserInfo, org string) {
	s.Printf("You are currently logged in with the following account: %q\n", userInfo.User)
	s.Printf("Your current organization: %q\n", org)

	if len(userInfo.Orgs) > 0 {
		s.printAvailableOrgsString(org, userInfo.Orgs)
	}
}

func (s *WhoAmICmd) printAvailableOrgsString(currentorg string, orgs []string) {
	s.Println("\nAvailable Organizations:")

	for _, org := range orgs {
		activeMarker := ""
		if currentorg == org {
			activeMarker = "*"
		}

		s.Printf("%s\t%s\n", activeMarker, org)
	}

	s.Printf("\nTo switch the organization use the following command:\n")
	s.Printf("$ nctl auth set-org <org-name>\n")
}
