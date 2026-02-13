package application

import (
	"context"
	"fmt"

	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/cli"
	corev1 "k8s.io/api/core/v1"
)

const (
	// BasicAuth key constants which represent the keys used in basic auth
	// secrets
	BasicAuthUsernameKey = "basicAuthUsername"
	BasicAuthPasswordKey = "basicAuthPassword"
)

// BasicAuth contains credentials for basic authentication
type BasicAuth struct {
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
}

// NewBasicAuthFromSecret returns a basic auth resource filled with data from
// a secret.
func NewBasicAuthFromSecret(ctx context.Context, secret meta.Reference, client *api.Client) (*BasicAuth, error) {
	basicAuthSecret := &corev1.Secret{}
	if err := client.Get(ctx, secret.NamespacedName(), basicAuthSecret); err != nil {
		return nil, fmt.Errorf("error when retrieving secret: %w", err)
	}
	if _, ok := basicAuthSecret.Data[BasicAuthUsernameKey]; !ok {
		return nil, cli.ErrorWithContext(fmt.Errorf("key %q not found in basic auth secret %q", BasicAuthUsernameKey, secret.Name)).
			WithExitCode(cli.ExitUsageError).
			WithContext("Secret", secret.Name).
			WithAvailable(BasicAuthUsernameKey, BasicAuthPasswordKey).
			WithSuggestions("Ensure the secret contains both basicAuthUsername and basicAuthPassword keys")
	}

	if _, ok := basicAuthSecret.Data[BasicAuthPasswordKey]; !ok {
		return nil, cli.ErrorWithContext(fmt.Errorf("key %q not found in basic auth secret %q", BasicAuthPasswordKey, secret.Name)).
			WithExitCode(cli.ExitUsageError).
			WithContext("Secret", secret.Name).
			WithAvailable(BasicAuthUsernameKey, BasicAuthPasswordKey).
			WithSuggestions("Ensure the secret contains both basicAuthUsername and basicAuthPassword keys")
	}

	return &BasicAuth{
		string(basicAuthSecret.Data[BasicAuthUsernameKey]),
		string(basicAuthSecret.Data[BasicAuthPasswordKey]),
	}, nil
}
