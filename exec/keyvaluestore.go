package exec

import (
	"context"
	"fmt"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/cli"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const kvsPort = "6380"

type kvsCmd struct {
	serviceCmd
}

// Help displays usage examples for the keyvaluestore exec command.
func (cmd kvsCmd) Help() string {
	return `Examples:
  # Connect to a KeyValueStore instance interactively
  nctl exec keyvaluestore mykvs

  # Pass extra flags to redis-cli (after --)
  nctl exec keyvaluestore mykvs -- --no-auth-warning
`
}

func (cmd *kvsCmd) Run(ctx context.Context, client *api.Client) error {
	kvs := &storage.KeyValueStore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}
	if err := client.Get(ctx, client.Name(cmd.Name), kvs); err != nil {
		return fmt.Errorf("getting keyvaluestore %q: %w", cmd.Name, err)
	}
	return connectAndExec(ctx, client, kvs, kvsConnector{}, cmd.serviceCmd)
}

// kvsConnector implements ServiceConnector for storage.KeyValueStore instances.
type kvsConnector struct{}

func (kvsConnector) Command() string { return "redis-cli" }

func (kvsConnector) Endpoint(kvs *storage.KeyValueStore) string {
	if kvs.Status.AtProvider.FQDN == "" {
		return ""
	}
	return kvs.Status.AtProvider.FQDN + ":" + kvsPort
}

func (kvsConnector) AllowedCIDRs(kvs *storage.KeyValueStore) []meta.IPv4CIDR {
	return kvs.Spec.ForProvider.AllowedCIDRs
}

func (kvsConnector) Update(ctx context.Context, client *api.Client, kvs *storage.KeyValueStore, cidrs []meta.IPv4CIDR) error {
	current := &storage.KeyValueStore{}
	if err := client.Get(ctx, api.ObjectName(kvs), current); err != nil {
		return err
	}

	if current.Spec.ForProvider.PublicNetworkingEnabled != nil && !*current.Spec.ForProvider.PublicNetworkingEnabled {
		return cli.ErrorWithContext(fmt.Errorf("public networking is disabled for keyvaluestore %q", kvs.GetName())).
			WithSuggestions(
				fmt.Sprintf("Enable it with: nctl update keyvaluestore %s --public-networking", kvs.GetName()),
			)
	}

	current.Spec.ForProvider.AllowedCIDRs = cidrs
	return client.Update(ctx, current)
}

func (kvsConnector) Args(kvs *storage.KeyValueStore, user, pw string) ([]string, func(), error) {
	caPath, cleanup, err := writeCACert(kvs.Status.AtProvider.CACert)
	if err != nil {
		return nil, func() {}, err
	}

	args := []string{
		"-h", kvs.Status.AtProvider.FQDN,
		"-p", kvsPort,
		"--tls",
		"-a", pw,
	}
	if caPath != "" {
		args = append(args, "--cacert", caPath)
	}

	return args, cleanup, nil
}
