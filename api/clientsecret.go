package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"

	"github.com/ninech/nctl/internal/cli"
)

// ClientSecretEnv is the environment variable providing the client secret of an APIServiceAccount.
// It takes precedence over a stored client secret.
// The client-credentials and login commands read it through their --client-secret flag.
const ClientSecretEnv = "NCTL_API_CLIENT_SECRET"

// ClientSecretKey returns the key of the client secret of clientID at tokenURL
// in the store of [Credentials.ClientSecrets].
func ClientSecretKey(clientID, tokenURL string) string {
	sum := sha256.Sum256([]byte(tokenURL + "\x00" + clientID))
	return hex.EncodeToString(sum[:])
}

// LookupClientSecret returns the client secret of clientID at tokenURL from store.
// Without a stored secret, the error explains how to provide one.
func LookupClientSecret(ctx context.Context, store SecretStore, clientID, tokenURL string) (string, error) {
	secret, err := store.Get(ctx, ClientSecretKey(clientID, tokenURL))
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf(
			"no client secret stored for client ID %q (%w), set %s or log in again with %q",
			clientID, err, ClientSecretEnv, cli.LoginCmd,
		)
	}
	if err != nil {
		return "", fmt.Errorf("reading the client secret: %w", err)
	}

	return secret, nil
}
