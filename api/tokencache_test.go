package api

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/int128/kubelogin/pkg/oidc"
	"github.com/int128/kubelogin/pkg/tokencache"
	"github.com/int128/kubelogin/pkg/tokencache/repository"
	"github.com/stretchr/testify/require"
)

// mapStore is an in-memory [SecretStore].
// The tests of package api cannot use the one of testutil, which imports api.
type mapStore map[string]string

func (s mapStore) Get(_ context.Context, key string) (string, error) {
	secret, ok := s[key]
	if !ok {
		return "", fmt.Errorf("%s: %w", key, fs.ErrNotExist)
	}
	return secret, nil
}

func (s mapStore) Set(_ context.Context, key, secret string) error {
	s[key] = secret
	return nil
}

func (s mapStore) Delete(_ context.Context, key string) error {
	if _, ok := s[key]; !ok {
		return fmt.Errorf("%s: %w", key, fs.ErrNotExist)
	}
	delete(s, key)
	return nil
}

var testTokens = oidc.TokenSet{IDToken: "id-token", RefreshToken: "refresh-token"}

func TestTokenCache(t *testing.T) {
	t.Parallel()
	is := require.New(t)
	store := mapStore{}
	cache := TokenCache{Store: store}
	key := TokenCacheKey("https://issuer.example.org", "client")

	_, err := cache.Find(t.Context(), key)
	is.ErrorIs(err, fs.ErrNotExist)

	is.NoError(cache.Save(t.Context(), key, testTokens))
	got, err := cache.Find(t.Context(), key)
	is.NoError(err)
	is.Equal(testTokens, *got)

	is.NoError(cache.Delete(t.Context(), key))
	is.ErrorIs(cache.Delete(t.Context(), key), fs.ErrNotExist)
}

func TestTokenCacheUsesTheFormatOfKubelogin(t *testing.T) {
	t.Parallel()
	is := require.New(t)
	config := tokencache.Config{Directory: t.TempDir()}
	key := TokenCacheKey("https://issuer.example.org", "client")
	is.NoError((&repository.Repository{}).Save(config, key, testTokens))

	// The keys are the file names of kubelogin and the values their contents.
	checksum, err := tokenCacheChecksum(key)
	is.NoError(err)
	file, err := os.ReadFile(filepath.Join(config.Directory, checksum))
	is.NoError(err, "the key must be the file name of kubelogin")

	store := mapStore{}
	is.NoError((TokenCache{Store: store}).Save(t.Context(), key, testTokens))
	is.Equal(string(file), store[checksum])

	got, err := (TokenCache{Store: mapStore{checksum: string(file)}}).Find(t.Context(), key)
	is.NoError(err)
	is.Equal(testTokens, *got)
}

func TestKubeloginRepository(t *testing.T) {
	t.Parallel()
	is := require.New(t)
	config := tokencache.Config{Directory: t.TempDir()}
	key := TokenCacheKey("https://issuer.example.org", "client")
	repo := kubeloginRepository{ctx: t.Context(), cache: TokenCache{Store: mapStore{}}}

	is.NoError(repo.Save(config, key, testTokens))
	got, err := repo.FindByKey(config, key)
	is.NoError(err)
	is.Equal(testTokens, *got)

	lock, err := repo.Lock(config, key)
	is.NoError(err)
	is.NoError(lock.Close())

	is.ErrorIs(repo.DeleteAll(config), errors.ErrUnsupported)
}

func TestLookupClientSecret(t *testing.T) {
	t.Parallel()
	is := require.New(t)
	store := mapStore{ClientSecretKey("id", "https://token.example.org"): "stored"}

	secret, err := LookupClientSecret(t.Context(), store, "id", "https://token.example.org")
	is.NoError(err)
	is.Equal("stored", secret)

	_, err = LookupClientSecret(t.Context(), store, "id", "https://other.example.org")
	is.ErrorIs(err, fs.ErrNotExist, "secrets are scoped to the token URL")
	is.ErrorContains(err, ClientSecretEnv)
}
