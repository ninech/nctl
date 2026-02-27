package auth

import (
	"context"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

// WhoAmIOutput is the serializable output of the whoami command.
type WhoAmIOutput struct {
	Account      string   `json:"account"`
	Organization string   `json:"organization"`
	Project      string   `json:"project"`
	Orgs         []string `json:"orgs,omitempty"`
}

type WhoAmICmd struct {
	format.Writer `hidden:""`
	APIURL        string `help:"URL of the Nine API." default:"https://nineapis.ch" env:"NCTL_API_URL" name:"api-url"`
	IssuerURL     string `help:"OIDC issuer URL of the API." default:"https://auth.nine.ch/auth/realms/pub"`
	ClientID      string `help:"OIDC client ID of the API." default:"nineapis.ch-f178254"`
	Format        string `help:"Configures output format. ${enum}" name:"output" short:"o" enum:"text,yaml,json" default:"text"`
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

	return cmd.printUserInfo(userInfo, org, client.Project)
}

func (cmd *WhoAmICmd) printUserInfo(userInfo *api.UserInfo, org, project string) error {
	switch cmd.Format {
	case "yaml":
		return format.PrettyPrintObject(WhoAmIOutput{
			Account:      userInfo.User,
			Organization: org,
			Project:      project,
			Orgs:         userInfo.Orgs,
		}, format.PrintOpts{Out: cmd.Writer, Format: format.OutputFormatTypeYAML})

	case "json":
		return format.PrettyPrintObject(WhoAmIOutput{
			Account:      userInfo.User,
			Organization: org,
			Project:      project,
			Orgs:         userInfo.Orgs,
		}, format.PrintOpts{Out: cmd.Writer, Format: format.OutputFormatTypeJSON})

	default:
		cmd.Printf("Account: %s\n", userInfo.User)
		cmd.Printf("Organization: %s\n", org)
		cmd.Printf("Project: %s\n", project)

		if len(userInfo.Orgs) > 0 {
			printAvailableOrgsString(cmd.Writer, org, userInfo.Orgs)
		}
	}

	return nil
}
