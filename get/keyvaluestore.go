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

type keyValueStoreCmd struct {
	Name       string `arg:"" help:"Name of the KeyValueStore Instance to get. If omitted all in the project will be listed." default:""`
	PrintToken bool   `help:"Print the bearer token of the Account. Requires name to be set." default:"false"`

	out io.Writer
}

func (cmd *keyValueStoreCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	cmd.out = defaultOut(cmd.out)

	keyValueStoreList := &storage.RedisList{}

	if err := get.list(ctx, client, keyValueStoreList, matchName(cmd.Name)); err != nil {
		return err
	}

	if len(keyValueStoreList.Items) == 0 {
		printEmptyMessage(cmd.out, storage.RedisKind, client.Project)
		return nil
	}

	if cmd.Name != "" && cmd.PrintToken {
		return cmd.printPassword(ctx, client, &keyValueStoreList.Items[0])
	}

	switch get.Output {
	case full:
		return cmd.printKeyValueStoreInstances(keyValueStoreList.Items, get, true)
	case noHeader:
		return cmd.printKeyValueStoreInstances(keyValueStoreList.Items, get, false)
	case yamlOut:
		return format.PrettyPrintObjects(keyValueStoreList.GetItems(), format.PrintOpts{})
	}

	return nil
}

func (cmd *keyValueStoreCmd) printKeyValueStoreInstances(list []storage.Redis, get *Cmd, header bool) error {
	w := tabwriter.NewWriter(cmd.out, 0, 0, 4, ' ', 0)

	if header {
		get.writeHeader(w, "NAME", "FQDN", "TLS", "MEMORY SIZE")
	}

	for _, keyValueStore := range list {
		get.writeTabRow(w, keyValueStore.Namespace, keyValueStore.Name, keyValueStore.Status.AtProvider.FQDN, "true", keyValueStore.Spec.ForProvider.MemorySize.String())
	}

	return w.Flush()
}

func (cmd *keyValueStoreCmd) printPassword(ctx context.Context, client *api.Client, keyValueStore *storage.Redis) error {
	pw, err := getConnectionSecret(ctx, client, storage.RedisUser, keyValueStore)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.out, pw)
	return nil
}
