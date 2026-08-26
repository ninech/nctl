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
	"github.com/int128/kubelogin/pkg/usecases/authentication/devicecode"
	"github.com/int128/kubelogin/pkg/usecases/authentication/ropc"
	"github.com/int128/kubelogin/pkg/usecases/credentialplugin"
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
	UseDeviceCodeArg         = "--use-device-code"
	CustomersPrefix          = "/Customers/"
	ClientCredentialsCmdName = "client-credentials"
	OIDCCmdName              = "oidc"
)

var (
	defaultBindAddresses = []string{"127.0.0.1:8000", "127.0.0.1:18000"}
	defaultAuthTimeout   = 180 * time.Second
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
		var usePKCE, useDeviceCode bool
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
			if arg == UseDeviceCodeArg {
				useDeviceCode = true
			}
		}
		if issuerURL == "" || clientID == "" {
			return "", fmt.Errorf("provided execConfig does not include expected args %s/%s", IssuerURLArg, ClientIDArg)
		}
		tk := DefaultTokenGetter{}
		return tk.GetTokenString(ctx, issuerURL, clientID, usePKCE, useDeviceCode)
	default:
		return "", fmt.Errorf("unknown exec command provided: %s", command)
	}
}

type TokenGetter interface {
	GetTokenString(ctx context.Context, issuerURL, clientID string, usePKCE, useDeviceCode bool) (string, error)
}

type DefaultTokenGetter struct{}

func (t *DefaultTokenGetter) GetTokenString(ctx context.Context, issuerURL, clientID string, usePKCE, useDeviceCode bool) (string, error) {
	buf := &bytes.Buffer{}
	if err := GetToken(ctx, issuerURL, clientID, usePKCE, useDeviceCode, buf); err != nil {
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

// GetToken executes the OIDC login flow using the kubelogin with the provided
// OIDC parameters writes the raw JSON ExecCredential result to out. If
// useDeviceCode is true, the OAuth 2.0 device authorization grant (RFC 8628)
// is used instead of the authorization code flow with a local browser and
// callback listener. This allows logging in from an environment without a
// local browser, e.g. an SSH session: the URL to approve the login is
// printed and can be opened on any other device.
func GetToken(ctx context.Context, issuerURL, clientID string, usePKCE, useDeviceCode bool, out io.Writer) error {
	grantOptionSet := authentication.GrantOptionSet{
		AuthCodeBrowserOption: &authcode.BrowserOption{
			BindAddress:           defaultBindAddresses,
			AuthenticationTimeout: defaultAuthTimeout,
		},
	}
	if useDeviceCode {
		grantOptionSet = authentication.GrantOptionSet{
			DeviceCodeOption: &devicecode.Option{
				SkipOpenBrowser: true,
			},
		}
	}

	in := credentialplugin.Input{
		Provider: oidc.Provider{
			IssuerURL: issuerURL,
			ClientID:  clientID,
		},
		TokenCacheConfig: tokencache.Config{
			Directory: path.Join(homedir.HomeDir(), DefaultTokenCachePath),
		},
		GrantOptionSet: grantOptionSet,
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
			DeviceCode: &devicecode.DeviceCode{
				Browser: &browser.Browser{},
				Logger:  logger,
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
