package auth

import (
	"context"
	"slices"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/config"
	"github.com/ninech/nctl/internal/format"
)

type SetOrgCmd struct {
	format.Writer `kong:"-"`
	Organization  string `arg:"" help:"Name of the organization to login to." default:""`
	APIURL        string `help:"URL of the Nine API." default:"https://nineapis.ch" env:"NCTL_API_URL" name:"api-url"`
	IssuerURL     string `help:"OIDC issuer URL of the API." default:"https://auth.nine.ch/auth/realms/pub"`
	ClientID      string `help:"OIDC client ID of the API." default:"nineapis.ch-f178254"`
}

func (cmd *SetOrgCmd) Run(ctx context.Context, client *api.Client) error {
	if cmd.Organization == "" {
		whoamicmd := WhoAmICmd{
			Writer:    cmd.Writer,
			APIURL:    cmd.APIURL,
			IssuerURL: cmd.IssuerURL,
			ClientID:  cmd.ClientID,
		}
		return whoamicmd.Run(ctx, client)
	}

	userInfo, err := api.GetUserInfoFromToken(client.Token(ctx))
	if err != nil {
		return err
	}

	if err := config.SetContextOrganization(client.KubeconfigPath, client.KubeconfigContext, cmd.Organization); err != nil {
		return err
	}

	cmd.Successf("📝", "set active Organization to %s", cmd.Organization)
	cmd.Println()

	// We only warn if the organization is not in the user's token, as RBAC
	// permissions in the API might still allow access even if the organization
	// is not listed in the JWT (e.g. for support staff or cross-org permissions).
	if !slices.Contains(userInfo.Orgs, cmd.Organization) {
		cmd.Warningf(
			"%s is not in list of available Organizations, you might not have access to all resources.\n",
			cmd.Organization,
		)
	}

	cmd.Successf("📝", "set active Organization to %s\n", cmd.Organization)
	return nil
}
