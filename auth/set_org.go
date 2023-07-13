package auth

import (
	"context"
	"net/url"

	"github.com/ninech/nctl/api"
)

type SetOrgCmd struct {
	Organization string `arg:"" help:"Name of the organization to login to."`
	APIURL       string `help:"The URL of the Nine API" default:"https://nineapis.ch" env:"NCTL_API_URL" name:"api-url"`
	IssuerURL    string `help:"Issuer URL is the OIDC issuer URL of the API." default:"https://auth.nine.ch/auth/realms/pub"`
	ClientID     string `help:"Client ID is the OIDC client ID of the API." default:"nineapis.ch-f178254"`
	ExecPlugin   bool   `help:"Automatically run exec plugin after writing the kubeconfig." hidden:"" default:"true"`
}

func (s *SetOrgCmd) Run(ctx context.Context, command string) error {
	loadingRules, err := api.LoadingRules()
	if err != nil {
		return err
	}

	apiURL, err := url.Parse(s.APIURL)
	if err != nil {
		return err
	}

	issuerURL, err := url.Parse(s.IssuerURL)
	if err != nil {
		return err
	}

	cfg, err := newAPIConfig(apiURL, issuerURL, command, s.ClientID, withOrganization(s.Organization))
	if err != nil {
		return err
	}

	return login(ctx, cfg, loadingRules.GetDefaultFilename(), s.Organization, runExecPlugin(s.ExecPlugin), project(s.Organization))
}
