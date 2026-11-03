package api_test

import (
	"errors"
	"os"
	"testing"

	"github.com/int128/kubelogin/pkg/oidc"
	"github.com/int128/kubelogin/pkg/tokencache/repository"
	"github.com/ninech/nctl/api"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
)

// These tests use the global keyring mock and environment, so they must not run in parallel.

var testTokens = oidc.TokenSet{IDToken: "id-token", RefreshToken: "refresh-token"}

// useHome points the home and configuration directories to a temporary directory.
func useHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", home+"/.config")
}

// useKeyring replaces the keyring with an empty mock failing with err, if set.
func useKeyring(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		keyring.MockInitWithError(err)
	} else {
		keyring.MockInit()
	}
	t.Cleanup(keyring.MockInit)
}

func TestTokensKeepSessionsOfKubelogin(t *testing.T) {
	is := require.New(t)
	useHome(t)
	useKeyring(t, nil)
	config := api.TokenCacheConfig()
	key := api.TokenCacheKey("https://issuer.example.org", "client")

	// A session cached on disk before nctl used the keyring.
	is.NoError((&repository.Repository{}).Save(config, key, testTokens))
	cache := api.TokenCache{Store: api.KeyringCredentials{}.Tokens(api.StorageKeyring)}
	got, err := cache.Find(t.Context(), key)
	is.NoError(err)
	is.Equal(testTokens, *got)

	// Refreshing the session moves it to the keyring.
	refreshed := oidc.TokenSet{IDToken: "new-id-token", RefreshToken: "new-refresh-token"}
	is.NoError(cache.Save(t.Context(), key, refreshed))
	_, err = (&repository.Repository{}).FindByKey(config, key)
	is.ErrorIs(err, os.ErrNotExist)
	got, err = cache.Find(t.Context(), key)
	is.NoError(err)
	is.Equal(refreshed, *got)
}

func TestTokensFallBackToFilesOfKubelogin(t *testing.T) {
	tests := map[string]struct {
		keyringErr error
		storage    api.Storage
	}{
		"keyring unavailable": {keyringErr: errors.New("no keyring"), storage: api.StorageKeyring},
		"insecure storage":    {storage: api.StorageFile},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			is := require.New(t)
			useHome(t)
			useKeyring(t, tt.keyringErr)
			key := api.TokenCacheKey("https://issuer.example.org", "client")

			cache := api.TokenCache{Store: api.KeyringCredentials{}.Tokens(tt.storage)}
			is.NoError(cache.Save(t.Context(), key, testTokens))

			// The file stays readable by kubelogin and earlier versions of nctl.
			got, err := (&repository.Repository{}).FindByKey(api.TokenCacheConfig(), key)
			is.NoError(err)
			is.Equal(testTokens, *got)
		})
	}
}

func TestTokensWarnOnFirstFallback(t *testing.T) {
	is := require.New(t)
	useHome(t)
	useKeyring(t, errors.New("no keyring"))
	var fallbacks []string
	creds := api.KeyringCredentials{OnFallback: func(path string, err error) {
		fallbacks = append(fallbacks, path+": "+err.Error())
	}}
	key := api.TokenCacheKey("https://issuer.example.org", "client")
	cache := api.TokenCache{Store: creds.Tokens(api.StorageKeyring)}

	is.NoError(cache.Save(t.Context(), key, testTokens))
	is.Len(fallbacks, 1)
	is.Contains(fallbacks[0], "no keyring")

	is.NoError(cache.Save(t.Context(), key, oidc.TokenSet{IDToken: "new-id-token", RefreshToken: "new-refresh-token"}))
	is.Len(fallbacks, 1, "refreshing tokens already in plain text does not warn again")

	fallbacks = nil
	is.NoError(api.TokenCache{Store: creds.Tokens(api.StorageFile)}.Save(t.Context(), api.TokenCacheKey("https://issuer.example.org", "other"), testTokens))
	is.Empty(fallbacks, "insecure storage is no fallback")
}

func TestClientSecretsWarnOnFallback(t *testing.T) {
	is := require.New(t)
	useHome(t)
	useKeyring(t, errors.New("no keyring"))
	var fallbacks []string
	creds := api.KeyringCredentials{OnFallback: func(path string, err error) {
		fallbacks = append(fallbacks, path+": "+err.Error())
	}}
	key := api.ClientSecretKey("id", "https://token.example.org")

	is.NoError(creds.ClientSecrets(api.StorageKeyring).Set(t.Context(), key, "secret"))
	is.Len(fallbacks, 1)
	is.Contains(fallbacks[0], key)
	is.Contains(fallbacks[0], "no keyring")

	is.NoError(creds.ClientSecrets(api.StorageKeyring).Set(t.Context(), key, "secret"))
	is.Len(fallbacks, 2, "every login falling back warns")

	secret, err := api.LookupClientSecret(t.Context(), creds.ClientSecrets(api.StorageKeyring), "id", "https://token.example.org")
	is.NoError(err)
	is.Equal("secret", secret)

	fallbacks = nil
	is.NoError(creds.ClientSecrets(api.StorageFile).Set(t.Context(), key, "secret"))
	is.Empty(fallbacks, "insecure storage is no fallback")
}

func TestClientSecretsUseKeyring(t *testing.T) {
	is := require.New(t)
	useHome(t)
	useKeyring(t, nil)
	key := api.ClientSecretKey("id", "https://token.example.org")

	is.NoError(api.KeyringCredentials{}.ClientSecrets(api.StorageKeyring).Set(t.Context(), key, "secret"))
	stored, err := keyring.Get("nctl", "client-secret/"+key)
	is.NoError(err)
	is.Equal("secret", stored)
}

func TestClientSecretsWithoutConfigDir(t *testing.T) {
	is := require.New(t)
	useKeyring(t, nil)
	t.Setenv("HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("AppData", "")

	// Creating the store succeeds, so commands without a login keep working.
	secrets := api.KeyringCredentials{}.ClientSecrets(api.StorageKeyring)
	_, err := secrets.Get(t.Context(), "key")
	is.ErrorContains(err, "finding the directory of the client secrets")
	is.Error(secrets.Set(t.Context(), "key", "secret"))
	is.Error(secrets.Delete(t.Context(), "key"))
}
