package auth

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/int128/kubelogin/pkg/oidc"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/testutil"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const testAPIHost = "api.example.org"

// newTokenServer returns the URL of a token endpoint issuing a service account token
// for the client secret "secret" only.
func newTokenServer(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, secret, _ := r.BasicAuth()
		if secret == "" {
			secret = r.FormValue("client_secret")
		}
		w.Header().Set("Content-Type", "application/json")
		if secret != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"invalid_client"}`))
			return
		}
		json.NewEncoder(w).Encode(&oauth2.Token{
			AccessToken: testutil.ASAJWTToken,
			TokenType:   "Bearer",
			ExpiresIn:   int64(time.Minute.Seconds()),
		})
	}))
	t.Cleanup(srv.Close)

	return srv.URL
}

// useKubeconfig points the kubeconfig to a new file in a temporary directory.
func useKubeconfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kubeconfig")
	t.Setenv(clientcmd.RecommendedConfigPathEnvVar, path)

	return path
}

// storages are the storages a login can choose, by test name.
var storages = map[string]api.Storage{"keyring": api.StorageKeyring, "insecure storage": api.StorageFile}

func TestLoginStoresClientSecret(t *testing.T) {
	for name, storage := range storages {
		t.Run(name, func(t *testing.T) {
			is := require.New(t)
			useKubeconfig(t)
			tokenURL := newTokenServer(t)
			creds := &testutil.Credentials{}

			cmd := &LoginCmd{
				API:             API{URL: "https://" + testAPIHost, ClientID: "id", ClientSecret: "secret", TokenURL: tokenURL},
				InsecureStorage: storage == api.StorageFile,
			}
			is.NoError(cmd.Run(t.Context(), creds))

			secret, err := creds.ClientSecrets(storage).Get(t.Context(), api.ClientSecretKey("id", tokenURL))
			is.NoError(err)
			is.Equal("secret", secret)
			is.Equal(1, creds.ClientSecretStore.Len()+creds.FileClientSecretStore.Len(), "only the chosen store holds the secret")
		})
	}
}

func TestLoginReusesStoredClientSecret(t *testing.T) {
	is := require.New(t)
	useKubeconfig(t)
	tokenURL := newTokenServer(t)
	creds := &testutil.Credentials{}
	is.NoError(creds.ClientSecretStore.Set(t.Context(), api.ClientSecretKey("id", tokenURL), "secret"))

	cmd := &LoginCmd{API: API{URL: "https://" + testAPIHost, ClientID: "id", TokenURL: tokenURL}}
	is.NoError(cmd.Run(t.Context(), creds))

	cmd = &LoginCmd{API: API{URL: "https://" + testAPIHost, ClientID: "unknown", TokenURL: tokenURL}}
	is.ErrorContains(cmd.Run(t.Context(), creds), `no client secret stored for client ID "unknown"`)
}

func TestGetTokenFromClientCredentialsExecConfig(t *testing.T) {
	is := require.New(t)
	useKubeconfig(t)
	tokenURL := newTokenServer(t)
	creds := &testutil.Credentials{}

	cmd := &LoginCmd{API: API{URL: "https://" + testAPIHost, ClientID: "id", ClientSecret: "secret", TokenURL: tokenURL}}
	is.NoError(cmd.Run(t.Context(), creds))
	exec := loadExecConfig(t)
	is.NotContains(exec.Args, api.ClientSecretArg+"secret")

	token, err := api.GetTokenFromExecConfig(t.Context(), exec, creds)
	is.NoError(err)
	is.Equal(testutil.ASAJWTToken, token)

	// Exec configs written by earlier versions carry the secret as an argument.
	is.NoError(creds.ClientSecretStore.Delete(t.Context(), api.ClientSecretKey("id", tokenURL)))
	legacy := exec.DeepCopy()
	legacy.Args = append(legacy.Args, api.ClientSecretArg+"secret")
	token, err = api.GetTokenFromExecConfig(t.Context(), legacy, creds)
	is.NoError(err)
	is.Equal(testutil.ASAJWTToken, token)

	_, err = api.GetTokenFromExecConfig(t.Context(), exec, creds)
	is.ErrorContains(err, "no client secret stored")
}

func TestGetTokenFromExecConfigPrefersClientSecretEnv(t *testing.T) {
	is := require.New(t)
	useKubeconfig(t)
	tokenURL := newTokenServer(t)
	creds := &testutil.Credentials{}

	cmd := &LoginCmd{API: API{URL: "https://" + testAPIHost, ClientID: "id", ClientSecret: "secret", TokenURL: tokenURL}}
	is.NoError(cmd.Run(t.Context(), creds))
	is.NoError(creds.ClientSecretStore.Set(t.Context(), api.ClientSecretKey("id", tokenURL), "outdated"))

	t.Setenv(api.ClientSecretEnv, "secret")
	token, err := api.GetTokenFromExecConfig(t.Context(), loadExecConfig(t), creds)
	is.NoError(err)
	is.Equal(testutil.ASAJWTToken, token)
}

func TestLogoutClientCredentials(t *testing.T) {
	for name, storage := range storages {
		t.Run(name, func(t *testing.T) {
			is := require.New(t)
			useKubeconfig(t)
			tokenURL := newTokenServer(t)
			creds := &testutil.Credentials{}

			login := &LoginCmd{
				API:             API{URL: "https://" + testAPIHost, ClientID: "id", ClientSecret: "secret", TokenURL: tokenURL},
				InsecureStorage: storage == api.StorageFile,
			}
			is.NoError(login.Run(t.Context(), creds))
			args, err := api.ParseExecArgs(loadExecConfig(t).Args)
			is.NoError(err)
			is.Equal(storage, args.Storage, "the exec plugin reads the store the login chose")

			out := &bytes.Buffer{}
			logout := &LogoutCmd{Writer: format.NewWriter(out, out), APIURL: "https://" + testAPIHost}
			is.NoError(logout.Run(t.Context(), creds))
			is.Contains(out.String(), "logged out from")
			_, err = creds.ClientSecrets(storage).Get(t.Context(), api.ClientSecretKey("id", tokenURL))
			is.ErrorIs(err, fs.ErrNotExist)

			out.Reset()
			is.NoError(logout.Run(t.Context(), creds))
			is.Contains(out.String(), "already logged out")
		})
	}
}

func TestLogoutClientCredentialsInKubeconfig(t *testing.T) {
	is := require.New(t)
	kubeconfig := useKubeconfig(t)
	is.NoError(clientcmd.WriteToFile(clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{testAPIHost: {Server: "https://" + testAPIHost}},
		Contexts: map[string]*clientcmdapi.Context{
			testAPIHost: {Cluster: testAPIHost, AuthInfo: testAPIHost},
			"other":     {Cluster: "other", AuthInfo: "other"},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			testAPIHost: {Exec: &clientcmdapi.ExecConfig{
				Args: []string{CmdName, ClientCredentialsCmdName, api.ClientIDArg + "id", api.ClientSecretArg + "secret", api.TokenURLArg + "https://token.example.org"},
			}},
			"other": {Token: "token"},
		},
		CurrentContext: testAPIHost,
	}, kubeconfig))

	out := &bytes.Buffer{}
	logout := &LogoutCmd{Writer: format.NewWriter(out, out), APIURL: "https://" + testAPIHost}
	is.NoError(logout.Run(t.Context(), &testutil.Credentials{}))
	is.Contains(out.String(), "logged out from")

	got, err := clientcmd.LoadFromFile(kubeconfig)
	is.NoError(err)
	is.NotContains(got.Contexts, testAPIHost)
	is.NotContains(got.AuthInfos, testAPIHost, "the client secret is removed")
	is.NotContains(got.Clusters, testAPIHost)
	is.Contains(got.Contexts, "other")
	is.Contains(got.AuthInfos, "other")
	is.Empty(got.CurrentContext)

	// Without the context, the OIDC login is the one to log out from.
	out.Reset()
	logout.IssuerURL, logout.ClientID = "https://issuer.example.org", "client"
	is.NoError(logout.Run(t.Context(), &testutil.Credentials{}))
	is.Contains(out.String(), "already logged out")
}

func TestLogoutOIDC(t *testing.T) {
	for name, storage := range storages {
		t.Run(name, func(t *testing.T) {
			is := require.New(t)
			useKubeconfig(t)
			creds := &testutil.Credentials{}

			var refreshToken string
			issuer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				refreshToken = r.FormValue("refresh_token")
			}))
			t.Cleanup(issuer.Close)

			login := &LoginCmd{
				API:                         API{URL: "https://" + testAPIHost},
				IssuerURL:                   issuer.URL,
				ClientID:                    "client",
				InsecureStorage:             storage == api.StorageFile,
				ForceInteractiveEnvOverride: true,
				tk:                          &fakeTokenGetter{},
			}
			is.NoError(login.Run(t.Context(), creds))

			// The fake token getter caches nothing, so cache the session here.
			cache := api.TokenCache{Store: creds.Tokens(storage)}
			key := api.TokenCacheKey(issuer.URL, "client")
			is.NoError(cache.Save(t.Context(), key, oidc.TokenSet{IDToken: testutil.FakeJWTToken, RefreshToken: "refresh"}))

			out := &bytes.Buffer{}
			logout := &LogoutCmd{
				Writer:    format.NewWriter(out, out),
				APIURL:    "https://" + testAPIHost,
				IssuerURL: issuer.URL,
				ClientID:  "client",
				tk:        &fakeTokenGetter{},
			}
			is.NoError(logout.Run(t.Context(), creds))
			is.Equal("refresh", refreshToken, "logout uses the store the login chose")
			is.Contains(out.String(), "logged out from")
			_, err := cache.Find(t.Context(), key)
			is.ErrorIs(err, fs.ErrNotExist)

			out.Reset()
			is.NoError(logout.Run(t.Context(), creds))
			is.Contains(out.String(), "already logged out")
		})
	}
}

func TestLogoutOIDCFailure(t *testing.T) {
	is := require.New(t)
	useKubeconfig(t)
	creds := &testutil.Credentials{}

	issuer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(issuer.Close)

	cache := api.TokenCache{Store: creds.Tokens(api.StorageKeyring)}
	key := api.TokenCacheKey(issuer.URL, "client")
	is.NoError(cache.Save(t.Context(), key, oidc.TokenSet{IDToken: testutil.FakeJWTToken, RefreshToken: "refresh"}))

	logout := &LogoutCmd{
		Writer:    format.NewWriter(&bytes.Buffer{}, &bytes.Buffer{}),
		APIURL:    "https://" + testAPIHost,
		IssuerURL: issuer.URL,
		ClientID:  "client",
		tk:        &fakeTokenGetter{},
	}
	is.ErrorContains(logout.Run(t.Context(), creds), "500")
	_, err := cache.Find(t.Context(), key)
	is.NoError(err, "a failed logout keeps the session")
}

// loadExecConfig returns the exec config of the API context of the kubeconfig.
func loadExecConfig(t *testing.T) *clientcmdapi.ExecConfig {
	t.Helper()
	kubeconfig, err := clientcmd.LoadFromFile(os.Getenv(clientcmd.RecommendedConfigPathEnvVar))
	require.NoError(t, err)
	authInfo := kubeconfig.AuthInfos[testAPIHost]
	require.NotNil(t, authInfo)
	require.NotNil(t, authInfo.Exec)

	return authInfo.Exec
}
