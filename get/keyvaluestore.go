package get

import (
	"context"
	"fmt"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type keyValueStoreCmd struct {
	resourceCmd
	PrintToken bool `help:"Print the bearer token of the Account. Requires name to be set." default:"false"`
}

func (cmd *keyValueStoreCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	return get.listPrint(ctx, client, cmd, api.MatchName(cmd.Name))
}

func (cmd *keyValueStoreCmd) list() client.ObjectList {
	return &storage.KeyValueStoreList{}
}

func (cmd *keyValueStoreCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	keyValueStoreList := list.(*storage.KeyValueStoreList)
	if len(keyValueStoreList.Items) == 0 {
		out.printEmptyMessage(storage.KeyValueStoreKind, client.Project)
		return nil
	}

	if cmd.Name != "" && cmd.PrintToken {
		return cmd.printPassword(ctx, client, &keyValueStoreList.Items[0], out)
	}

	switch out.Format {
	case full:
		return cmd.printKeyValueStoreInstances(keyValueStoreList.Items, out, true)
	case noHeader:
		return cmd.printKeyValueStoreInstances(keyValueStoreList.Items, out, false)
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

func (cmd *keyValueStoreCmd) printKeyValueStoreInstances(list []storage.KeyValueStore, out *output, header bool) error {
	if header {
		out.writeHeader("NAME", "FQDN", "TLS", "MEMORY SIZE")
	}

	for _, keyValueStore := range list {
		out.writeTabRow(keyValueStore.Namespace, keyValueStore.Name, keyValueStore.Status.AtProvider.FQDN, "true", keyValueStore.Spec.ForProvider.MemorySize.String())
	}

	return out.tabWriter.Flush()
}

func (cmd *keyValueStoreCmd) printPassword(ctx context.Context, client *api.Client, keyValueStore *storage.KeyValueStore, out *output) error {
	pw, err := getConnectionSecret(ctx, client, storage.KeyValueStoreUser, keyValueStore)
	if err != nil {
		return err
	}

	fmt.Fprintln(out.writer, pw)
	return nil
}
