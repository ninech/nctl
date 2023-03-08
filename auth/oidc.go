package auth

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path"
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
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/homedir"
)

type OIDCCmd struct {
	IssuerURL     string
	ClientID      string
	ClientSecret  string
	ExtraScopes   []string
	UsePKCE       bool
	TokenCacheDir string
}

const (
	OIDCCmdName           = "auth oidc"
	defaultTokenCachePath = ".kube/cache/oidc-login"
)

var (
	defaultBindAddresses = []string{"127.0.0.1:8000", "127.0.0.1:18000"}
	defaultAuthTimeout   = 180 * time.Second
)

func (o *OIDCCmd) Run(ctx context.Context) error {
	in := credentialplugin.Input{
		Provider: oidc.Provider{
			IssuerURL:    o.IssuerURL,
			ClientID:     o.ClientID,
			ClientSecret: o.ClientSecret,
			UsePKCE:      o.UsePKCE,
			ExtraScopes:  o.ExtraScopes,
		},
		TokenCacheDir: o.TokenCacheDir,
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
			Stdout: os.Stdout,
		},
		Mutex: &mutex.Mutex{
			Logger: logger,
		},
	}

	if in.TokenCacheDir == "" {
		in.TokenCacheDir = path.Join(homedir.HomeDir(), defaultTokenCachePath)
	}

	if err := getToken.Do(ctx, in); err != nil {
		return fmt.Errorf("error getting OIDC token: %w", err)
	}

	return nil
}

// execConfig returns an *clientcmdapi.ExecConfig that can be used to login to
// a kubernetes cluster using nctl.
func execConfig(command, clientID string, issuerURL *url.URL) *clientcmdapi.ExecConfig {
	return &clientcmdapi.ExecConfig{
		APIVersion: "client.authentication.k8s.io/v1beta1",
		Command:    command,
		Args: []string{
			"auth",
			"oidc",
			"--issuer-url=" + issuerURL.String(),
			"--client-id=" + clientID,
			"--use-pkce",
		},
	}
}
