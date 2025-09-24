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
	PrintToken  bool `help:"Print the bearer token of the Account. Requires name to be set." xor:"print"`
	PrintCACert bool `help:"Print the ca certificate. Requires name to be set." xor:"print"`
}

func (cmd *keyValueStoreCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	return get.listPrint(ctx, client, cmd, api.MatchName(cmd.Name))
}

func (cmd *keyValueStoreCmd) list() client.ObjectList {
	return &storage.KeyValueStoreList{}
}

func (cmd *keyValueStoreCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	keyValueStoreList, ok := list.(*storage.KeyValueStoreList)
	if !ok {
		return fmt.Errorf("expected %T, got %T", &storage.KeyValueStoreList{}, list)
	}
	if len(keyValueStoreList.Items) == 0 {
		return out.printEmptyMessage(storage.KeyValueStoreKind, client.Project)
	}

	if cmd.Name != "" && cmd.PrintToken {
		return cmd.printSecret(out.writer, ctx, client, &keyValueStoreList.Items[0], func(_, pw string) string { return pw })
	}
	if cmd.Name != "" && cmd.PrintCACert {
		return printBase64(out.writer, keyValueStoreList.Items[0].Status.AtProvider.CACert)
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
		out.writeHeader("NAME", "LOCATION", "VERSION", "PRIVATE FQDN", "PUBLIC FQDN", "MEMORY POLICY", "MEMORY SIZE")
	}

	for _, kvs := range list {
		out.writeTabRow(
			kvs.Namespace,
			kvs.Name,
			string(kvs.Spec.ForProvider.Location),
			string(kvs.Spec.ForProvider.Version),
			kvs.Status.AtProvider.PrivateNetworkingFQDN,
			kvs.Status.AtProvider.FQDN,
			string(kvs.Spec.ForProvider.MaxMemoryPolicy),
			kvs.Spec.ForProvider.MemorySize.String(),
		)
	}

	return out.tabWriter.Flush()
}
