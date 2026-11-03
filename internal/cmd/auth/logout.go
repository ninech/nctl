package auth

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"strings"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/kubeconfig"
	"k8s.io/client-go/tools/clientcmd"
)

type LogoutCmd struct {
	format.Writer `hidden:""`
	APIURL        string `help:"URL of the Nine API." default:"https://nineapis.ch" env:"NCTL_API_URL" name:"api-url"`
	IssuerURL     string `help:"OIDC issuer URL of the API." default:"https://auth.nine.ch/auth/realms/pub"`
	ClientID      string `help:"OIDC client ID of the API." default:"nineapis.ch-f178254"`
	tk            api.TokenGetter
}

// Run logs out and removes the tokens or client secret of the login from creds.
// The login is the one of the exec plugin of the kubeconfig context of the API,
// or else the OIDC login of --issuer-url and --client-id.
func (l *LogoutCmd) Run(ctx context.Context, creds api.Credentials) error {
	apiURL, err := url.Parse(l.APIURL)
	if err != nil {
		return err
	}
	loadingRules, err := api.LoadingRules()
	if err != nil {
		return err
	}
	contextName := apiURL.Host

	args, ok, err := execArgs(loadingRules, contextName)
	if err != nil {
		return err
	}
	if !ok {
		return l.logoutOIDC(ctx, creds, api.OIDCLogin{IssuerURL: l.IssuerURL, ClientID: l.ClientID})
	}

	switch args.Command {
	case api.ClientCredentialsCmdName:
		if args.ClientSecret != "" {
			return l.logoutKubeconfigClientSecret(loadingRules, contextName)
		}
		return l.logoutClientCredentials(ctx, args, creds.ClientSecrets(args.Storage))
	default:
		return l.logoutOIDC(ctx, creds, args.OIDCLogin())
	}
}

// execArgs returns the arguments of the exec plugin with which the kubeconfig context
// contextName authenticates. ok is false if the context does not use an exec plugin.
func execArgs(loadingRules *clientcmd.ClientConfigLoadingRules, contextName string) (args api.ExecArgs, ok bool, err error) {
	config, err := loadingRules.Load()
	if err != nil {
		return api.ExecArgs{}, false, err
	}

	kubeContext, ok := config.Contexts[contextName]
	if !ok {
		return api.ExecArgs{}, false, nil
	}
	authInfo, ok := config.AuthInfos[kubeContext.AuthInfo]
	if !ok || authInfo.Exec == nil {
		return api.ExecArgs{}, false, nil
	}

	args, err = api.ParseExecArgs(authInfo.Exec.Args)
	if err != nil {
		return api.ExecArgs{}, false, fmt.Errorf("reading the login of the kubeconfig context %q: %w", contextName, err)
	}

	return args, true, nil
}

// logoutKubeconfigClientSecret removes the kubeconfig context contextName,
// whose exec plugin carries the client secret as an argument, as the secret can not be removed on its own.
func (l *LogoutCmd) logoutKubeconfigClientSecret(configAccess clientcmd.ConfigAccess, contextName string) error {
	if err := kubeconfig.RemoveContext(configAccess, contextName); err != nil {
		return fmt.Errorf("removing the kubeconfig context %q holding the client secret: %w", contextName, err)
	}

	l.Successf("🗑", "removed %s and its client secret from the kubeconfig", contextName)
	l.Successf("👋", "logged out from %s", l.APIURL)

	return nil
}

// logoutClientCredentials removes the client secret of args from secrets.
func (l *LogoutCmd) logoutClientCredentials(ctx context.Context, args api.ExecArgs, secrets api.SecretStore) error {
	if err := secrets.Delete(ctx, api.ClientSecretKey(args.ClientID, args.TokenURL)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			l.Failuref("🤔", "seems like you are already logged out from %s", l.APIURL)
			return nil
		}
		return fmt.Errorf("removing the client secret: %w", err)
	}

	l.Successf("👋", "logged out from %s", l.APIURL)

	return nil
}

// logoutOIDC ends the session of login and removes its tokens from creds.
func (l *LogoutCmd) logoutOIDC(ctx context.Context, creds api.Credentials, login api.OIDCLogin) error {
	cache := api.TokenCache{Store: creds.Tokens(login.Storage)}
	key := api.TokenCacheKey(login.IssuerURL, login.ClientID)

	tokens, err := cache.Find(ctx, key)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			l.Failuref("🤔", "seems like you are already logged out from %s", l.APIURL)
			return nil
		}
		return err
	}

	form := url.Values{}
	form.Add("client_id", login.ClientID)
	form.Add("refresh_token", tokens.RefreshToken)

	logoutEndpoint := strings.Join(
		[]string{login.IssuerURL, "protocol", "openid-connect", "logout"},
		"/",
	)

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		logoutEndpoint,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	token, err := l.tokenGetter(creds).GetTokenString(ctx, login)
	if err != nil {
		return fmt.Errorf("error getting token: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+token)

	c := http.DefaultClient
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("error logging out from OIDC: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http request error %d to %s", resp.StatusCode, logoutEndpoint)
	}

	if err := cache.Delete(ctx, key); err != nil {
		return fmt.Errorf("error removing the local cache: %w", err)
	}

	l.Successf("👋", "logged out from %s", l.APIURL)

	return nil
}

func (l *LogoutCmd) tokenGetter(creds api.Credentials) api.TokenGetter {
	if l.tk != nil {
		return l.tk
	}
	return api.NewTokenGetter(creds)
}
