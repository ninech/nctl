package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	kresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type keyValueStoreCmd struct {
	Name            string                                `arg:"" default:"" help:"Name of the KeyValueStore instance to update."`
	MemorySize      *string                               `help:"MemorySize configures KeyValueStore to use a specified amount of memory for the data set." placeholder:"1Gi"`
	MaxMemoryPolicy *storage.KeyValueStoreMaxMemoryPolicy `help:"MaxMemoryPolicy specifies the exact behavior KeyValueStore follows when the maxmemory limit is reached." placeholder:"allkeys-lru"`
	AllowedCidrs    *[]storage.IPv4CIDR                   `help:"AllowedCIDRs specify the allowed IP addresses, connecting to the instance." placeholder:"0.0.0.0/0"`
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

func (cmd *keyValueStoreCmd) applyUpdates(keyValueStore *storage.KeyValueStore) error {
	if cmd.MemorySize != nil {
		q, err := kresource.ParseQuantity(*cmd.MemorySize)
		if err != nil {
			return fmt.Errorf("error parsing memory size %q: %w", *cmd.MemorySize, err)
		}

		keyValueStore.Spec.ForProvider.MemorySize = &storage.KeyValueStoreMemorySize{Quantity: q}
	}
	if cmd.MaxMemoryPolicy != nil {
		keyValueStore.Spec.ForProvider.MaxMemoryPolicy = *cmd.MaxMemoryPolicy
	}
	if cmd.AllowedCidrs != nil {
		keyValueStore.Spec.ForProvider.AllowedCIDRs = *cmd.AllowedCidrs
	}

	return nil
}
