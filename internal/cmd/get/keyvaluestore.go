package get

import (
	"context"
	"fmt"
	"strconv"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type keyValueStoreCmd struct {
	ServiceCmd
	// PrintToken is deprecated in favour of PrintPassword and only kept for backwards compatibility.
	PrintToken bool `help:"Deprecated: use --print-password." hidden:"" xor:"print"`
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

	if cmd.PrintToken {
		cmd.PrintPassword = true
	}

	return cmd.run(ctx, client, out, keyValueStoreList, service{
		kind:             storage.KeyValueStoreKind,
		connectionString: cmd.connectionString,
		printList:        cmd.printKeyValueStoreInstances,
		caCert: func(mg resource.Managed) (string, error) {
			kvs, ok := mg.(*storage.KeyValueStore)
			if !ok {
				return "", fmt.Errorf("expected %T, got %T", &storage.KeyValueStore{}, mg)
			}
			return kvs.Status.AtProvider.CACert, nil
		},
	})
}

// connectionString returns a Redis URI with TLS, which is also understood by Valkey clients.
// See https://www.iana.org/assignments/uri-schemes/prov/rediss
func (cmd *keyValueStoreCmd) connectionString(mg resource.Managed, user, password string) (string, error) {
	kvs, ok := mg.(*storage.KeyValueStore)
	if !ok {
		return "", fmt.Errorf("expected %T, got %T", &storage.KeyValueStore{}, mg)
	}

	port := strconv.Itoa(int(storage.KeyValueStorePort))
	return connectionURI("rediss", kvs.Status.AtProvider.FQDN, port, user, password, "", nil)
}

func (cmd *keyValueStoreCmd) printKeyValueStoreInstances(resources resource.ManagedList, out *output, header bool) error {
	list, ok := resources.(*storage.KeyValueStoreList)
	if !ok {
		return fmt.Errorf("expected %T, got %T", &storage.KeyValueStoreList{}, resources)
	}

	if header {
		out.writeHeader("NAME", "LOCATION", "VERSION", "PRIVATE FQDN", "PUBLIC FQDN", "MEMORY POLICY", "MEMORY SIZE")
	}

	for _, kvs := range list.Items {
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
