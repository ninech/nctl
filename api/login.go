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

	"github.com/int128/kubelogin/pkg/credentialplugin/writer"
	"github.com/int128/kubelogin/pkg/infrastructure/browser"
	"github.com/int128/kubelogin/pkg/infrastructure/clock"
	"github.com/int128/kubelogin/pkg/infrastructure/logger"
	"github.com/int128/kubelogin/pkg/infrastructure/mutex"
	"github.com/int128/kubelogin/pkg/infrastructure/reader"
	"github.com/int128/kubelogin/pkg/oidc"
	"github.com/int128/kubelogin/pkg/oidc/client"
	"github.com/int128/kubelogin/pkg/tlsclientconfig/loader"
	"github.com/int128/kubelogin/pkg/tokencache/repository"
	"github.com/int128/kubelogin/pkg/usecases/authentication"
	"github.com/int128/kubelogin/pkg/usecases/authentication/authcode"
	"github.com/int128/kubelogin/pkg/usecases/authentication/ropc"
	"github.com/int128/kubelogin/pkg/usecases/credentialplugin"
	"k8s.io/client-go/pkg/apis/clientauthentication"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/homedir"
)

const (
	defaultTokenCachePath = ".kube/cache/oidc-login"
	IssuerURLArg          = "--issuer-url="
	ClientIDArg           = "--client-id="
	UsePKCEArg            = "--use-pkce"
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
	var issuerURL, clientID string
	var usePKCE bool
	for _, arg := range execConfig.Args {
		if strings.HasPrefix(arg, IssuerURLArg) {
			issuerURL = strings.TrimPrefix(arg, IssuerURLArg)
		}
		if strings.HasPrefix(arg, ClientIDArg) {
			clientID = strings.TrimPrefix(arg, ClientIDArg)
		}
		if arg == UsePKCEArg {
			usePKCE = true
		}
	}

	if len(issuerURL) == 0 || len(clientID) == 0 {
		return "", fmt.Errorf("provided execConfig does not include expected args %s/%s", IssuerURLArg, ClientIDArg)
	}

	buf := &bytes.Buffer{}
	if err := GetToken(ctx, issuerURL, clientID, usePKCE, buf); err != nil {
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
// OIDC parameters writes the raw JSON ExecCredential result to out.
func GetToken(ctx context.Context, issuerURL, clientID string, usePKCE bool, out io.Writer) error {
	in := credentialplugin.Input{
		Provider: oidc.Provider{
			IssuerURL: issuerURL,
			ClientID:  clientID,
			UsePKCE:   usePKCE,
		},
		TokenCacheDir: path.Join(homedir.HomeDir(), defaultTokenCachePath),
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
			Clock:  clockReal,
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
		Logger:               logger,
		TokenCacheRepository: &repository.Repository{},
		Writer: &writer.Writer{
			Stdout: out,
		},
		Mutex: &mutex.Mutex{
			Logger: logger,
		},
	}

	if err := getToken.Do(ctx, in); err != nil {
		return fmt.Errorf("error getting OIDC token: %w", err)
	}

	return nil
}
