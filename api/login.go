package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
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
	"github.com/int128/kubelogin/pkg/tokencache"
	"github.com/int128/kubelogin/pkg/tokencache/repository"
	"github.com/int128/kubelogin/pkg/usecases/authentication"
	"github.com/int128/kubelogin/pkg/usecases/authentication/authcode"
	"github.com/int128/kubelogin/pkg/usecases/authentication/ropc"
	"github.com/int128/kubelogin/pkg/usecases/credentialplugin"
	"github.com/ninech/nctl/internal/format"
	"golang.org/x/oauth2/clientcredentials"
	"k8s.io/client-go/pkg/apis/clientauthentication"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/homedir"
)

const (
	DefaultTokenCachePath    = ".kube/cache/oidc-login"
	IssuerURLArg             = "--issuer-url="
	ClientIDArg              = "--client-id="
	ClientSecretArg          = "--client-secret="
	TokenURLArg              = "--token-url="
	UsePKCEArg               = "--use-pkce"
	CustomersPrefix          = "/Customers/"
	ClientCredentialsCmdName = "client-credentials"
	OIDCCmdName              = "oidc"
)

var (
	defaultBindAddresses = []string{"127.0.0.1:8000", "127.0.0.1:18000"}
	defaultAuthTimeout   = 180 * time.Second
	// tokenExpiryLeeway is added to the current time when checking the exp
	// claim of an access token. This treats tokens that are about to expire as
	// already expired so we refresh them before they are rejected in-flight by
	// a resource server.
	tokenExpiryLeeway = 10 * time.Second
)

// GetTokenFromConfig takes a rest.Config and returns a valid OIDC access
// token or the static bearer token if it's set in the config.
func GetTokenFromConfig(ctx context.Context, cfg *rest.Config) (string, error) {
	if len(cfg.BearerToken) != 0 {
		return cfg.BearerToken, nil
	}

	if cfg.ExecProvider == nil {
		return "", fmt.Errorf("config does not contain execProvider")
	}

	return GetTokenFromExecConfig(ctx, cfg.ExecProvider)
}

// GetTokenFromExecConfig takes the provided execConfig, parses out the args
// and gets the token by executing the login flow.
func GetTokenFromExecConfig(ctx context.Context, execConfig *api.ExecConfig) (string, error) {
	if len(execConfig.Args) < 2 {
		return "", fmt.Errorf("provided execConfig args are invalid, expected at least two")
	}

	command := execConfig.Args[1]
	switch command {
	case ClientCredentialsCmdName:
		cfg := &clientcredentials.Config{}
		for _, arg := range execConfig.Args {
			if after, ok := strings.CutPrefix(arg, ClientIDArg); ok {
				cfg.ClientID = after
			}
			if after, ok := strings.CutPrefix(arg, ClientSecretArg); ok {
				cfg.ClientSecret = after
			}
			if after, ok := strings.CutPrefix(arg, TokenURLArg); ok {
				cfg.TokenURL = after
			}
		}
		if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.TokenURL == "" {
			return "", fmt.Errorf("provided execConfig does not include expected args %s/%s/%s", ClientIDArg, ClientSecretArg, TokenURLArg)
		}
		token, err := cfg.Token(ctx)
		if err != nil {
			return "", err
		}
		return token.AccessToken, nil
	case OIDCCmdName:
		var issuerURL, clientID string
		var usePKCE bool
		for _, arg := range execConfig.Args {
			if after, ok := strings.CutPrefix(arg, IssuerURLArg); ok {
				issuerURL = after
			}
			if after, ok := strings.CutPrefix(arg, ClientIDArg); ok {
				clientID = after
			}
			if arg == UsePKCEArg {
				usePKCE = true
			}
		}
		if issuerURL == "" || clientID == "" {
			return "", fmt.Errorf("provided execConfig does not include expected args %s/%s", IssuerURLArg, ClientIDArg)
		}
		tk := DefaultTokenGetter{}
		return tk.GetTokenString(ctx, issuerURL, clientID, usePKCE)
	default:
		return "", fmt.Errorf("unknown exec command provided: %s", command)
	}
}

type TokenGetter interface {
	GetTokenString(ctx context.Context, issuerURL, clientID string, usePKCE bool) (string, error)
}

type DefaultTokenGetter struct{}

func (t *DefaultTokenGetter) GetTokenString(ctx context.Context, issuerURL, clientID string, usePKCE bool) (string, error) {
	return validToken(ctx, issuerURL, clientID, usePKCE, time.Now(), getTokenString)
}

// tokenFetcher obtains a token (and the expiry reported by kubelogin) for the
// given OIDC parameters. forceRefresh makes kubelogin bypass its token cache.
// It is a separate type so the orchestration in validToken can be tested
// without driving the real kubelogin login flow.
type tokenFetcher func(ctx context.Context, issuerURL, clientID string, usePKCE, forceRefresh bool) (string, *time.Time, error)

// validToken fetches a token and guarantees it is not expired. If the fetched
// token is already expired it forces a refresh, and if even the refreshed
// token is expired it returns an actionable error instead of a stale token.
func validToken(ctx context.Context, issuerURL, clientID string, usePKCE bool, now time.Time, fetch tokenFetcher) (string, error) {
	token, kubeExpiry, err := fetch(ctx, issuerURL, clientID, usePKCE, false)
	if err != nil {
		return "", err
	}

	// kubelogin only refreshes the token when its own cache considers it
	// expired (e.g. based on a stored ExpirationTimestamp). That can hand us a
	// token whose JWT exp has already passed while the cache still thinks it's
	// valid. Inspect the exp claim ourselves and, if the token is stale, force
	// kubelogin to refresh instead of returning it.
	if _, expired := accessTokenExpired(token, kubeExpiry, now); !expired {
		return token, nil
	}

	token, kubeExpiry, err = fetch(ctx, issuerURL, clientID, usePKCE, true)
	if err != nil {
		return "", err
	}

	// If the forced refresh still yields an expired token, a non-interactive
	// refresh was not possible (e.g. the refresh token is gone). Fail with an
	// actionable error instead of writing a stale token to stdout.
	if expiry, expired := accessTokenExpired(token, kubeExpiry, now); expired {
		return "", fmt.Errorf(
			"access token expired on %s, run %q to re-authenticate",
			expiry.UTC().Format(time.RFC3339), format.Command().Login(),
		)
	}

	return token, nil
}

// getTokenString runs the OIDC login flow via kubelogin and returns the access
// token together with the expiry reported by kubelogin (nil if absent). When
// forceRefresh is true, kubelogin bypasses its token cache and refreshes.
func getTokenString(ctx context.Context, issuerURL, clientID string, usePKCE, forceRefresh bool) (string, *time.Time, error) {
	buf := &bytes.Buffer{}
	if err := GetToken(ctx, issuerURL, clientID, usePKCE, forceRefresh, buf); err != nil {
		return "", nil, err
	}

	creds := &clientauthentication.ExecCredential{}
	if err := json.NewDecoder(buf).Decode(creds); err != nil {
		return "", nil, fmt.Errorf("unable to decode exec credentials: %w", err)
	}

	var kubeExpiry *time.Time
	if creds.Status.ExpirationTimestamp != nil {
		kubeExpiry = &creds.Status.ExpirationTimestamp.Time
	}

	return creds.Status.Token, kubeExpiry, nil
}

// accessTokenExpired reports whether the access token is expired as of now. It
// prefers the exp claim of the JWT itself and falls back to the expiry
// reported by kubelogin for non-JWT tokens. The returned time is the expiry
// the decision was based on (zero if no expiry information is available, in
// which case the token is treated as not expired).
func accessTokenExpired(token string, kubeExpiry *time.Time, now time.Time) (time.Time, bool) {
	if exp, ok := tokenExpiry(token); ok {
		return exp, !exp.After(now.Add(tokenExpiryLeeway))
	}
	if kubeExpiry != nil {
		return *kubeExpiry, !kubeExpiry.After(now.Add(tokenExpiryLeeway))
	}
	return time.Time{}, false
}

// tokenExpiry decodes the (unverified) JWT and returns the time encoded in its
// exp claim. The signature is intentionally not verified: we only need the
// expiry to decide whether to force a refresh. ok is false if the token is not
// a parseable JWT or carries no exp claim.
func tokenExpiry(token string) (expiry time.Time, ok bool) {
	claims := jwt.RegisteredClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(token, &claims); err != nil {
		return time.Time{}, false
	}
	if claims.ExpiresAt == nil {
		return time.Time{}, false
	}
	return claims.ExpiresAt.Time, true
}

// GetToken executes the OIDC login flow using the kubelogin with the provided
// OIDC parameters writes the raw JSON ExecCredential result to out. When
// forceRefresh is true, kubelogin bypasses its token cache and refreshes the
// token regardless of its cached expiration.
func GetToken(ctx context.Context, issuerURL, clientID string, usePKCE, forceRefresh bool, out io.Writer) error {
	in := credentialplugin.Input{
		Provider: oidc.Provider{
			IssuerURL: issuerURL,
			ClientID:  clientID,
		},
		ForceRefresh: forceRefresh,
		TokenCacheConfig: tokencache.Config{
			Directory: path.Join(homedir.HomeDir(), DefaultTokenCachePath),
		},
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
		TokenCacheRepository:   &repository.Repository{},
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
