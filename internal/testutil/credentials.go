package testutil

import (
	"context"
	"fmt"
	"io/fs"
	"sync"

	"github.com/ninech/nctl/api"
)

// SecretStore is an in-memory [api.SecretStore].
type SecretStore struct {
	mu      sync.Mutex
	secrets map[string]string
}

var _ api.SecretStore = (*SecretStore)(nil)

func (s *SecretStore) Get(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	secret, ok := s.secrets[key]
	if !ok {
		return "", fmt.Errorf("secret %q: %w", key, fs.ErrNotExist)
	}

	return secret, nil
}

func (s *SecretStore) Set(_ context.Context, key, secret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.secrets == nil {
		s.secrets = map[string]string{}
	}
	s.secrets[key] = secret

	return nil
}

func (s *SecretStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.secrets[key]; !ok {
		return fmt.Errorf("secret %q: %w", key, fs.ErrNotExist)
	}
	delete(s.secrets, key)

	return nil
}

// Len returns the number of secrets in the store.
func (s *SecretStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.secrets)
}

// Credentials are in-memory [api.Credentials].
// The stores of [api.StorageFile] are separate from the others,
// so tests can tell which one a login used.
type Credentials struct {
	TokenStore, FileTokenStore               SecretStore
	ClientSecretStore, FileClientSecretStore SecretStore
}

var _ api.Credentials = (*Credentials)(nil)

func (c *Credentials) Tokens(storage api.Storage) api.SecretStore {
	if storage == api.StorageFile {
		return &c.FileTokenStore
	}

	return &c.TokenStore
}

func (c *Credentials) ClientSecrets(storage api.Storage) api.SecretStore {
	if storage == api.StorageFile {
		return &c.FileClientSecretStore
	}

	return &c.ClientSecretStore
}
