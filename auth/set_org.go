package auth

import (
	"context"

	"github.com/ninech/nctl/api"
)

type SetOrgCmd struct {
	Organization string `arg:"" help:"Name of the organization to login to." default:""`
	APIURL       string `help:"The URL of the Nine API" default:"https://nineapis.ch" env:"NCTL_API_URL" name:"api-url"`
	IssuerURL    string `help:"Issuer URL is the OIDC issuer URL of the API." default:"https://auth.nine.ch/auth/realms/pub"`
	ClientID     string `help:"Client ID is the OIDC client ID of the API." default:"nineapis.ch-f178254"`
}

func (s *SetOrgCmd) Run(ctx context.Context, client *api.Client) error {
	if s.Organization == "" {
		whoamicmd := WhoAmICmd{APIURL: s.APIURL, IssuerURL: s.IssuerURL, ClientID: s.ClientID}
		return whoamicmd.Run(ctx, client)
	}

	return SetContextOrganization(client.KubeconfigPath, client.KubeconfigContext, s.Organization)
}
