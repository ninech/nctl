package get

import (
	"context"
	"os"
	"text/tabwriter"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type redisCmd struct{}

func (cmd *redisCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	redisList := &storage.RedisList{}

	if err := get.list(ctx, client, redisList); err != nil {
		return err
	}

	if len(redisList.Items) == 0 {
		printEmptyMessage(os.Stdout, storage.RedisKind, client.Project)
		return nil
	}

	switch get.Output {
	case full:
		return printRedisInstances(redisList.Items, get, true)
	case noHeader:
		return printRedisInstances(redisList.Items, get, false)
	case yamlOut:
		return format.PrettyPrintObjects(redisList.GetItems(), format.PrintOpts{})
	}

	return nil
}

func printRedisInstances(list []storage.Redis, get *Cmd, header bool) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)

	if header {
		get.writeHeader(w, "NAME", "FQDN", "TLS", "MEMORY SIZE")
	}

	for _, redis := range list {
		get.writeTabRow(w, redis.Namespace, redis.Name, redis.Status.AtProvider.FQDN, "true", redis.Spec.ForProvider.MemorySize.String())
	}

	return w.Flush()
}
