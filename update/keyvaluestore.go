package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type keyValueStoreCmd struct {
	resourceCmd
	MemorySize              *storage.KeyValueStoreMemorySize      `placeholder:"${keyvaluestore_memorysize_default}" help:"MemorySize configures KeyValueStore to use a specified amount of memory for the data set."`
	MaxMemoryPolicy         *storage.KeyValueStoreMaxMemoryPolicy `placeholder:"${keyvaluestore_maxmemorypolicy_default}" help:"MaxMemoryPolicy specifies the exact behavior KeyValueStore follows when the maxmemory limit is reached."`
	AllowedCidrs            *[]meta.IPv4CIDR                      `placeholder:"203.0.113.1/32" help:"AllowedCIDRs specify the allowed IP addresses, connecting to the instance."`
	PublicNetworkingEnabled *bool                                 `placeholder:"true" help:"Specifies if the service should be available without service connection."`
}

func (cmd *keyValueStoreCmd) Run(ctx context.Context, client *api.Client) error {
	keyValueStore := &storage.KeyValueStore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}

	return newUpdater(client, keyValueStore, storage.KeyValueStoreKind, func(current resource.Managed) error {
		keyValueStore, ok := current.(*storage.KeyValueStore)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, storage.KeyValueStore{})
		}

		return cmd.applyUpdates(keyValueStore)
	}).Update(ctx)
}

func (cmd *keyValueStoreCmd) applyUpdates(kvs *storage.KeyValueStore) error {
	if cmd.MemorySize != nil {
		kvs.Spec.ForProvider.MemorySize = cmd.MemorySize
	}
	if cmd.MaxMemoryPolicy != nil {
		kvs.Spec.ForProvider.MaxMemoryPolicy = *cmd.MaxMemoryPolicy
	}
	if cmd.AllowedCidrs != nil {
		kvs.Spec.ForProvider.AllowedCIDRs = *cmd.AllowedCidrs
	}
	if cmd.PublicNetworkingEnabled != nil {
		kvs.Spec.ForProvider.PublicNetworkingEnabled = cmd.PublicNetworkingEnabled
	}

	return nil
}
