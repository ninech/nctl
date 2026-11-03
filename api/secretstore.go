package api

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ninech/nctl/api/internal/credential"
)

// SecretStore keeps secrets by key.
// Get and Delete return an error matching [io/fs.ErrNotExist] if the store holds no secret for the key.
type SecretStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, secret string) error
	Delete(ctx context.Context, key string) error
}

// Storage selects where a [SecretStore] writes its secrets.
// Stores of every Storage read the secrets written with [StorageFile].
type Storage int

const (
	// StorageKeyring writes secrets to the keyring of the operating system.
	StorageKeyring Storage = iota
	// StorageFile writes secrets to plain text files only the user can read.
	StorageFile
)

func (s Storage) String() string {
	switch s {
	case StorageKeyring:
		return "keyring"
	case StorageFile:
		return "file"
	default:
		return fmt.Sprintf("Storage(%d)", int(s))
	}
}

// Credentials provides the stores of the tokens and client secrets of the logins.
type Credentials interface {
	// Tokens returns the store caching the OIDC tokens under the keys of [TokenCache].
	Tokens(Storage) SecretStore
	// ClientSecrets returns the store of the client secrets of APIServiceAccounts
	// under the keys of [ClientSecretKey].
	ClientSecrets(Storage) SecretStore
}

// KeyringCredentials stores the tokens and client secrets of the logins
// in the keyring of the operating system, falling back to files only the user can read.
// Its token files are those of kubelogin, so sessions kubelogin cached keep working.
// The zero value is ready to use and is the default of [New].
type KeyringCredentials struct {
	// OnFallback, if set, is called when a client secret or token is written to the file at path
	// because the keyring failed with err.
	// It is called on every such write of a client secret, which only a login writes,
	// but only on the first of a token, which every refresh writes.
	OnFallback func(path string, err error)
}

var _ Credentials = KeyringCredentials{}

// Tokens returns the store of the OIDC tokens.
func (c KeyringCredentials) Tokens(storage Storage) SecretStore {
	return credential.New(
		TokenCacheConfig().Directory,
		credential.WithNamespace("oidc-login"),
		credential.WithInsecure(storage == StorageFile),
		credential.WithFallbackHandler(func(path string, created bool, err error) {
			if created && c.OnFallback != nil {
				c.OnFallback(path, err)
			}
		}),
	)
}

// ClientSecrets returns the store of the client secrets,
// whose files are below the user configuration directory, see [os.UserConfigDir].
// If there is none, every operation of the store fails.
func (c KeyringCredentials) ClientSecrets(storage Storage) SecretStore {
	dir, err := os.UserConfigDir()
	if err != nil {
		return failingStore{fmt.Errorf("finding the directory of the client secrets: %w", err)}
	}

	return credential.New(
		filepath.Join(dir, Name, "client-secrets"),
		credential.WithNamespace("client-secret"),
		credential.WithInsecure(storage == StorageFile),
		credential.WithFallbackHandler(func(path string, _ bool, err error) {
			if c.OnFallback != nil {
				c.OnFallback(path, err)
			}
		}),
	)
}

// failingStore is a [SecretStore] whose operations fail with err.
type failingStore struct {
	err error
}

func (s failingStore) Get(context.Context, string) (string, error) { return "", s.err }
func (s failingStore) Set(context.Context, string, string) error   { return s.err }
func (s failingStore) Delete(context.Context, string) error        { return s.err }
