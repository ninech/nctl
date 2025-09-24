package auth

import (
	"context"
	"fmt"
	"slices"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/config"
	"github.com/ninech/nctl/internal/format"
)

type SetOrgCmd struct {
	Organization string `arg:"" help:"Name of the organization to login to." default:""`
	APIURL       string `help:"URL of the Nine API." default:"https://nineapis.ch" env:"NCTL_API_URL" name:"api-url"`
	IssuerURL    string `help:"OIDC issuer URL of the API." default:"https://auth.nine.ch/auth/realms/pub"`
	ClientID     string `help:"OIDC client ID of the API." default:"nineapis.ch-f178254"`
}

func (s *SetOrgCmd) Run(ctx context.Context, client *api.Client) error {
	if s.Organization == "" {
		whoamicmd := WhoAmICmd{APIURL: s.APIURL, IssuerURL: s.IssuerURL, ClientID: s.ClientID}
		return whoamicmd.Run(ctx, client)
	}

	userInfo, err := api.GetUserInfoFromToken(client.Token(ctx))
	if err != nil {
		return err
	}

	if err := config.SetContextOrganization(client.KubeconfigPath, client.KubeconfigContext, s.Organization); err != nil {
		return err
	}

	if !slices.Contains(userInfo.Orgs, s.Organization) {
		format.PrintWarningf("%s is not in list of available Organizations, you might not have access to all resources.\n", s.Organization)
	}

	fmt.Println(format.SuccessMessagef("📝", "set active Organization to %s", s.Organization))
	return nil
}
