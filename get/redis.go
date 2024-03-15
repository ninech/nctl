package get

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type redisCmd struct {
	Name       string `arg:"" help:"Name of the Redis Instance to get. If omitted all in the project will be listed." default:""`
	PrintToken bool   `help:"Print the bearer token of the Account. Requires name to be set." default:"false"`

	out io.Writer
}

func (cmd *redisCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	cmd.out = defaultOut(cmd.out)

	redisList := &storage.RedisList{}

	if err := get.list(ctx, client, redisList, matchName(cmd.Name)); err != nil {
		return err
	}

	if len(redisList.Items) == 0 {
		printEmptyMessage(cmd.out, storage.RedisKind, client.Project)
		return nil
	}

	if cmd.Name != "" && cmd.PrintToken {
		return cmd.printPassword(ctx, client, &redisList.Items[0])
	}

	switch get.Output {
	case full:
		return cmd.printRedisInstances(redisList.Items, get, true)
	case noHeader:
		return cmd.printRedisInstances(redisList.Items, get, false)
	case yamlOut:
		return format.PrettyPrintObjects(redisList.GetItems(), format.PrintOpts{})
	}

	return nil
}

func (cmd *redisCmd) printRedisInstances(list []storage.Redis, get *Cmd, header bool) error {
	w := tabwriter.NewWriter(cmd.out, 0, 0, 4, ' ', 0)

	if header {
		get.writeHeader(w, "NAME", "FQDN", "TLS", "MEMORY SIZE")
	}

	for _, redis := range list {
		get.writeTabRow(w, redis.Namespace, redis.Name, redis.Status.AtProvider.FQDN, "true", redis.Spec.ForProvider.MemorySize.String())
	}

	return w.Flush()
}

func (cmd *redisCmd) printPassword(ctx context.Context, client *api.Client, redis *storage.Redis) error {
	pw, err := getConnectionSecret(ctx, client, "default", redis)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.out, pw)
	return nil
}
