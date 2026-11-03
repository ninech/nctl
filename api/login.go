package api

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	credreader "github.com/int128/kubelogin/pkg/credentialplugin/reader"
	"github.com/int128/kubelogin/pkg/credentialplugin/writer"
	"github.com/int128/kubelogin/pkg/infrastructure/browser"
	"github.com/int128/kubelogin/pkg/infrastructure/clock"
	"github.com/int128/kubelogin/pkg/infrastructure/logger"
	"github.com/int128/kubelogin/pkg/infrastructure/reader"
	"github.com/int128/kubelogin/pkg/oidc"
	"github.com/int128/kubelogin/pkg/oidc/client"
	"github.com/int128/kubelogin/pkg/tlsclientconfig/loader"
	"github.com/int128/kubelogin/pkg/usecases/authentication"
	"github.com/int128/kubelogin/pkg/usecases/authentication/authcode"
	"github.com/int128/kubelogin/pkg/usecases/authentication/ropc"
	"github.com/int128/kubelogin/pkg/usecases/credentialplugin"
	"golang.org/x/oauth2/clientcredentials"
	"k8s.io/client-go/pkg/apis/clientauthentication"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
)

const (
	IssuerURLArg             = "--issuer-url="
	ClientIDArg              = "--client-id="
	ClientSecretArg          = "--client-secret="
	TokenURLArg              = "--token-url="
	UsePKCEArg               = "--use-pkce"
	InsecureStorageArg       = "--insecure-storage"
	CustomersPrefix          = "/Customers/"
	AuthCmdName              = "auth"
	ClientCredentialsCmdName = "client-credentials"
	OIDCCmdName              = "oidc"
)

var (
	defaultBindAddresses = []string{"127.0.0.1:8000", "127.0.0.1:18000"}
	defaultAuthTimeout   = 180 * time.Second
)

// GetTokenFromConfig takes a rest.Config and returns a valid OIDC access
// token or the static bearer token if it's set in the config.
// The tokens and client secrets of the login are kept in credentials.
func GetTokenFromConfig(ctx context.Context, cfg *rest.Config, credentials Credentials) (string, error) {
	if len(cfg.BearerToken) != 0 {
		return cfg.BearerToken, nil
	}

	if cfg.ExecProvider == nil {
		return "", fmt.Errorf("config does not contain execProvider")
	}

	return GetTokenFromExecConfig(ctx, cfg.ExecProvider, credentials)
}

// GetTokenFromExecConfig runs the login of the exec plugin of execConfig in process
// and returns its token.
// The tokens and client secrets of the login are kept in credentials.
// A client secret in [ClientSecretEnv] takes precedence over a stored one.
func GetTokenFromExecConfig(ctx context.Context, execConfig *api.ExecConfig, credentials Credentials) (string, error) {
	args, err := ParseExecArgs(execConfig.Args)
	if err != nil {
		return "", err
	}

	if args.Command == OIDCCmdName {
		return NewTokenGetter(credentials).GetTokenString(ctx, args.OIDCLogin())
	}

	secret := cmp.Or(args.ClientSecret, os.Getenv(ClientSecretEnv))
	if secret == "" {
		secret, err = LookupClientSecret(ctx, credentials.ClientSecrets(args.Storage), args.ClientID, args.TokenURL)
		if err != nil {
			return "", err
		}
	}
	cfg := clientcredentials.Config{ClientID: args.ClientID, ClientSecret: secret, TokenURL: args.TokenURL}
	token, err := cfg.Token(ctx)
	if err != nil {
		return "", err
	}

	return token.AccessToken, nil
}

// OIDCLogin configures an OIDC login.
type OIDCLogin struct {
	IssuerURL string
	ClientID  string
	UsePKCE   bool
	// Storage is where the login caches its tokens.
	Storage Storage
}

// TokenGetter gets the token of an OIDC login.
type TokenGetter interface {
	GetTokenString(ctx context.Context, login OIDCLogin) (string, error)
}

// NewTokenGetter returns a [TokenGetter] running [GetToken]
// with the tokens cached in credentials.
func NewTokenGetter(credentials Credentials) TokenGetter {
	return tokenGetter{credentials: credentials}
}

type tokenGetter struct {
	credentials Credentials
}

func (t tokenGetter) GetTokenString(ctx context.Context, login OIDCLogin) (string, error) {
	buf := &bytes.Buffer{}
	if err := GetToken(ctx, login, t.credentials, buf); err != nil {
		return "", err
	}

	creds := &clientauthentication.ExecCredential{}
	if err := json.NewDecoder(buf).Decode(creds); err != nil {
		return "", fmt.Errorf("unable to decode exec credentials: %w", err)
	}

	if creds.Status.ExpirationTimestamp != nil && creds.Status.ExpirationTimestamp.Time.Before(time.Now()) {
		return "", fmt.Errorf("token expired on %s", creds.Status.ExpirationTimestamp.Time)
	}

	return creds.Status.Token, nil
}

// GetToken executes the OIDC login flow using kubelogin
// and writes the raw JSON ExecCredential result to out.
// The tokens are cached in credentials.Tokens(login.Storage).
func GetToken(ctx context.Context, login OIDCLogin, credentials Credentials, out io.Writer) error {
	in := credentialplugin.Input{
		Provider: oidc.Provider{
			IssuerURL: login.IssuerURL,
			ClientID:  login.ClientID,
		},
		TokenCacheConfig: TokenCacheConfig(),
		GrantOptionSet: authentication.GrantOptionSet{
			AuthCodeBrowserOption: &authcode.BrowserOption{
				BindAddress:           defaultBindAddresses,
				AuthenticationTimeout: defaultAuthTimeout,
			},
		},
	}

	clockReal := &clock.Real{}
	stdin := os.Stdin
	logger := logger.New()

	getToken := credentialplugin.GetToken{
		Authentication: &authentication.Authentication{
			ClientFactory: &client.Factory{
				Loader: loader.Loader{},
				Clock:  clockReal,
				Logger: logger,
			},
			Logger: logger,
			AuthCodeBrowser: &authcode.Browser{
				Browser: &browser.Browser{},
				Logger:  logger,
			},
			AuthCodeKeyboard: &authcode.Keyboard{
				Reader: &reader.Reader{
					Stdin: stdin,
				},
				Logger: logger,
			},
			ROPC: &ropc.ROPC{
				Reader: &reader.Reader{
					Stdin: stdin,
				},
				Logger: logger,
			},
		},
		Logger:                 logger,
		TokenCacheRepository:   kubeloginRepository{ctx: ctx, cache: TokenCache{Store: credentials.Tokens(login.Storage)}},
		CredentialPluginReader: &credreader.Reader{},
		CredentialPluginWriter: &writer.Writer{
			Stdout: out,
		},
		Clock: &clock.Real{},
	}
	if err := getToken.Do(ctx, in); err != nil {
		return fmt.Errorf("error getting OIDC token: %w", err)
	}

	return nil
}

type UserInfo struct {
	User    string
	Orgs    []string
	Project string
}

func GetUserInfoFromToken(tokenString string) (*UserInfo, error) {
	type authClaims struct {
		Email        string   `json:"email"`
		Groups       []string `json:"groups"`
		Sub          string   `json:"sub"`
		Organization string   `json:"organization"`
		Project      string   `json:"project"`
		jwt.RegisteredClaims
	}
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, &authClaims{})
	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT token: %v", err)
	}

	claims, ok := token.Claims.(*authClaims)
	if !ok {
		return nil, fmt.Errorf("failed to parse JWT claims: %v", err)
	}

	var orgs []string
	if claims.Organization != "" {
		orgs = append(orgs, claims.Organization)
	}
	for _, grp := range claims.Groups {
		if after, ok := strings.CutPrefix(grp, CustomersPrefix); ok {
			orgs = append(orgs, after)
		}
	}

	acc := claims.Email

	if acc == "" {
		acc = claims.Sub
	}

	return &UserInfo{
		User:    acc,
		Orgs:    orgs,
		Project: claims.Project,
	}, nil
}
