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
	resourceCmd
	PrintToken bool `help:"Print the bearer token of the Account. Requires name to be set." default:"false"`

	out io.Writer
}

func (cmd *keyValueStoreCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	cmd.out = defaultOut(cmd.out)

	keyValueStoreList := &storage.KeyValueStoreList{}

	if err := get.list(ctx, client, keyValueStoreList, api.MatchName(cmd.Name)); err != nil {
		return err
	}

	if len(keyValueStoreList.Items) == 0 {
		get.printEmptyMessage(cmd.out, storage.KeyValueStoreKind, client.Project)
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
	case jsonOut:
		return format.PrettyPrintObjects(
			keyValueStoreList.GetItems(),
			format.PrintOpts{
				Format: format.OutputFormatTypeJSON,
				JSONOpts: format.JSONOutputOptions{
					PrintSingleItem: cmd.Name != "",
				},
			})
	}

	return nil
}

func (cmd *keyValueStoreCmd) printKeyValueStoreInstances(list []storage.KeyValueStore, get *Cmd, header bool) error {
	w := tabwriter.NewWriter(cmd.out, 0, 0, 4, ' ', 0)

	if header {
		get.writeHeader(w, "NAME", "FQDN", "TLS", "MEMORY SIZE")
	}

	for _, keyValueStore := range list {
		get.writeTabRow(w, keyValueStore.Namespace, keyValueStore.Name, keyValueStore.Status.AtProvider.FQDN, "true", keyValueStore.Spec.ForProvider.MemorySize.String())
	}

	return w.Flush()
}

func (cmd *keyValueStoreCmd) printPassword(ctx context.Context, client *api.Client, keyValueStore *storage.KeyValueStore) error {
	pw, err := getConnectionSecret(ctx, client, storage.KeyValueStoreUser, keyValueStore)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.out, pw)
	return nil
}
