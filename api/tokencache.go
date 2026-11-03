package api

import (
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/int128/kubelogin/pkg/oidc"
	"github.com/int128/kubelogin/pkg/tokencache"
	"github.com/int128/kubelogin/pkg/tokencache/repository"
	"k8s.io/client-go/util/homedir"
)

// DefaultTokenCachePath is the directory below the home directory
// holding the token cache of kubelogin.
const DefaultTokenCachePath = ".kube/cache/oidc-login"

// TokenCacheConfig returns the token cache configuration of nctl.
// Its directory holds the locks of the token cache and the files kubelogin cached tokens in.
func TokenCacheConfig() tokencache.Config {
	return tokencache.Config{Directory: filepath.Join(homedir.HomeDir(), DefaultTokenCachePath)}
}

// TokenCacheKey returns the key under which kubelogin caches the tokens of
// the OIDC client clientID at issuerURL.
func TokenCacheKey(issuerURL, clientID string) tokencache.Key {
	return tokencache.Key{Provider: oidc.Provider{IssuerURL: issuerURL, ClientID: clientID}}
}

// TokenCache keeps OIDC tokens in a [SecretStore].
// Its keys and values are the file names and contents kubelogin uses for its token cache,
// so a store whose files are those below the directory of [TokenCacheConfig]
// keeps the sessions kubelogin cached.
type TokenCache struct {
	Store SecretStore
}

// tokenCacheEntry is the format of a cached token set, the same as kubelogin uses.
type tokenCacheEntry struct {
	IDToken      string `json:"id_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

// Find returns the cached token set of key.
// It returns an error matching [io/fs.ErrNotExist] if there is none.
func (c TokenCache) Find(ctx context.Context, key tokencache.Key) (*oidc.TokenSet, error) {
	checksum, err := tokenCacheChecksum(key)
	if err != nil {
		return nil, err
	}
	data, err := c.Store.Get(ctx, checksum)
	if err != nil {
		return nil, fmt.Errorf("reading the token cache: %w", err)
	}
	var entry tokenCacheEntry
	if err := json.Unmarshal([]byte(data), &entry); err != nil {
		return nil, fmt.Errorf("invalid token cache: %w", err)
	}

	return &oidc.TokenSet{IDToken: entry.IDToken, RefreshToken: entry.RefreshToken}, nil
}

// Save caches tokenSet under key.
func (c TokenCache) Save(ctx context.Context, key tokencache.Key, tokenSet oidc.TokenSet) error {
	checksum, err := tokenCacheChecksum(key)
	if err != nil {
		return err
	}
	data, err := json.Marshal(tokenCacheEntry{IDToken: tokenSet.IDToken, RefreshToken: tokenSet.RefreshToken})
	if err != nil {
		return err
	}

	return c.Store.Set(ctx, checksum, string(data))
}

// Delete removes the cached token set of key.
// It returns an error matching [io/fs.ErrNotExist] if there is none.
func (c TokenCache) Delete(ctx context.Context, key tokencache.Key) error {
	checksum, err := tokenCacheChecksum(key)
	if err != nil {
		return err
	}

	return c.Store.Delete(ctx, checksum)
}

// kubeloginRepository adapts a [TokenCache] to the token cache interface of kubelogin.
// That interface has no context, so the adapter carries the one of the login it serves.
type kubeloginRepository struct {
	ctx   context.Context
	cache TokenCache
}

var _ repository.Interface = kubeloginRepository{}

func (r kubeloginRepository) FindByKey(_ tokencache.Config, key tokencache.Key) (*oidc.TokenSet, error) {
	return r.cache.Find(r.ctx, key)
}

func (r kubeloginRepository) Save(_ tokencache.Config, key tokencache.Key, tokenSet oidc.TokenSet) error {
	return r.cache.Save(r.ctx, key, tokenSet)
}

// Lock locks the token cache entry of key against concurrent logins with a file, which holds no secret.
func (r kubeloginRepository) Lock(config tokencache.Config, key tokencache.Key) (io.Closer, error) {
	return (&repository.Repository{}).Lock(config, key)
}

// DeleteAll is not supported, as the store may hold other secrets.
func (r kubeloginRepository) DeleteAll(tokencache.Config) error {
	return errors.ErrUnsupported
}

// tokenCacheChecksum returns the name kubelogin gives the cache file of key.
// It follows computeChecksum of github.com/int128/kubelogin/pkg/tokencache/repository.
func tokenCacheChecksum(key tokencache.Key) (string, error) {
	s := sha256.New()
	if err := gob.NewEncoder(s).Encode(&key); err != nil {
		return "", fmt.Errorf("could not encode the token cache key: %w", err)
	}

	return hex.EncodeToString(s.Sum(nil)), nil
}
