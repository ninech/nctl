package auth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"

	"github.com/alecthomas/kong"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/kubeconfig"
)

const (
	defaultClientID  = "nineapis.ch-f178254"
	defaultIssuerURL = "https://auth.nine.ch/auth/realms/pub"
	defaultTokenURL  = defaultIssuerURL + "/protocol/openid-connect/token"
)

type LoginCmd struct {
	format.Writer               `hidden:""`
	API                         API    `embed:"" prefix:"api-"`
	Organization                string `help:"Name of your organization to use when providing an API client ID/secret." env:"NCTL_ORGANIZATION"`
	IssuerURL                   string `help:"OIDC issuer URL of the API." default:"${issuer_url}" hidden:""`
	ClientID                    string `help:"OIDC client ID of the API." default:"${client_id}" hidden:""`
	InsecureStorage             bool   `help:"Save credentials in plain text files instead of the keyring of the operating system."`
	ForceInteractiveEnvOverride bool   `help:"Used for internal purposes only. Set to true to force interactive environment explicit override. Set to false to fall back to automatic interactivity detection." default:"false" hidden:""`
	tk                          api.TokenGetter
}

const ErrNonInteractiveEnvironmentEmptyToken = "a static API token is required in non-interactive environments"

// Run logs in and writes the kubeconfig context of the API.
// The tokens or client secret of the login are kept in creds.
func (cmd *LoginCmd) Run(ctx context.Context, creds api.Credentials) error {
	apiURL, err := url.Parse(cmd.API.URL)
	if err != nil {
		return err
	}

	issuerURL, err := url.Parse(cmd.IssuerURL)
	if err != nil {
		return err
	}

	loadingRules, err := api.LoadingRules()
	if err != nil {
		return err
	}

	command, err := os.Executable()
	if err != nil {
		return fmt.Errorf("can not identify executable path: %w", err)
	}

	if cmd.API.Token != "" {
		if cmd.Organization == "" {
			return fmt.Errorf("you need to set the --organization parameter explicitly if you use --api-token")
		}
		userInfo, err := api.GetUserInfoFromToken(cmd.API.Token)
		if err != nil {
			return err
		}
		cfg, err := kubeconfig.NewAPIConfig(apiURL, issuerURL, command, cmd.ClientID, kubeconfig.UseStaticToken(cmd.API.Token), kubeconfig.WithOrganization(cmd.Organization))
		if err != nil {
			return err
		}
		return kubeconfig.Login(cmd.Writer, cfg, loadingRules.GetDefaultFilename(), userInfo.User, "", kubeconfig.WithProject(cmd.Organization))
	}

	if cmd.API.ClientID != "" {
		secrets := creds.ClientSecrets(cmd.storage())
		secret, err := cmd.API.clientSecret(ctx, secrets)
		if err != nil {
			return err
		}
		userInfo, err := cmd.API.UserInfo(ctx, secret)
		if err != nil {
			return err
		}
		if cmd.Organization == "" && len(userInfo.Orgs) == 0 {
			return fmt.Errorf("unable to find organization, you need to set the --organization parameter explicitly")
		}
		org := cmd.Organization
		if org == "" {
			org = userInfo.Orgs[0]
		}
		// A secret read from the store is stored already.
		if cmd.API.ClientSecret != "" {
			if err := secrets.Set(ctx, api.ClientSecretKey(cmd.API.ClientID, cmd.API.TokenURL), cmd.API.ClientSecret); err != nil {
				return fmt.Errorf("storing the client secret: %w", err)
			}
		}
		cfg, err := kubeconfig.NewAPIConfig(apiURL, issuerURL, command, cmd.API.ClientID, kubeconfig.UseClientCredentials(cmd.API.clientCredentials()), kubeconfig.WithOrganization(org), kubeconfig.WithStorage(cmd.storage()))
		if err != nil {
			return err
		}
		proj := org
		if userInfo.Project != "" {
			proj = userInfo.Project
		}
		return kubeconfig.Login(cmd.Writer, cfg, loadingRules.GetDefaultFilename(), userInfo.User, "", kubeconfig.WithProject(proj))
	}

	if !cmd.ForceInteractiveEnvOverride && !format.IsInteractiveEnvironment(os.Stdout) {
		return errors.New(ErrNonInteractiveEnvironmentEmptyToken)
	}

	token, err := cmd.tokenGetter(creds).GetTokenString(ctx, api.OIDCLogin{
		IssuerURL: cmd.IssuerURL,
		ClientID:  cmd.ClientID,
		UsePKCE:   true,
		Storage:   cmd.storage(),
	})
	if err != nil {
		return err
	}

	userInfo, err := api.GetUserInfoFromToken(token)
	if err != nil {
		return err
	}

	if len(userInfo.Orgs) == 0 {
		return fmt.Errorf("error getting an organization for the account %q. Please contact support", userInfo.User)
	}

	org := userInfo.Orgs[0]
	if len(userInfo.Orgs) > 1 {
		cmd.Infof("", "Multiple organizations found for the account %q.", userInfo.User)
		cmd.Infof("", "Defaulting to %q", org)
		printAvailableOrgsString(cmd.Writer, org, userInfo.Orgs)
	}

	cfg, err := kubeconfig.NewAPIConfig(apiURL, issuerURL, command, cmd.ClientID, kubeconfig.WithOrganization(org), kubeconfig.WithStorage(cmd.storage()))
	if err != nil {
		return err
	}

	return kubeconfig.Login(cmd.Writer, cfg, loadingRules.GetDefaultFilename(), userInfo.User, "", kubeconfig.WithProject(org))
}

func printAvailableOrgsString(w format.Writer, currentorg string, orgs []string) {
	w.Println("\nAvailable Organizations:")

	for _, org := range orgs {
		activeMarker := ""
		if currentorg == org {
			activeMarker = "*"
		}

		w.Printf("%s\t%s\n", activeMarker, org)
	}

	w.Println()
}

func (cmd *LoginCmd) tokenGetter(creds api.Credentials) api.TokenGetter {
	if cmd.tk != nil {
		return cmd.tk
	}
	return api.NewTokenGetter(creds)
}

// storage returns where the login keeps its tokens or client secret.
func (cmd *LoginCmd) storage() api.Storage {
	return storage(cmd.InsecureStorage)
}

// storage returns the storage selected by --insecure-storage.
func storage(insecure bool) api.Storage {
	if insecure {
		return api.StorageFile
	}

	return api.StorageKeyring
}

// LoginKongVars returns all variables which are used in the login command
func LoginKongVars() kong.Vars {
	result := make(kong.Vars)
	result["client_id"] = defaultClientID
	result["issuer_url"] = defaultIssuerURL
	result["token_url"] = defaultTokenURL
	return result
}
