package create

import (
	"context"
	"fmt"
	"time"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type keyValueStoreCmd struct {
	Name            string                               `arg:"" default:"" help:"Name of the KeyValueStore instance. A random name is generated if omitted."`
	Location        string                               `default:"nine-es34" help:"Location where the KeyValueStore instance is created."`
	MemorySize      string                               `help:"MemorySize configures KeyValueStore to use a specified amount of memory for the data set." placeholder:"1Gi"`
	MaxMemoryPolicy storage.KeyValueStoreMaxMemoryPolicy `help:"MaxMemoryPolicy specifies the exact behavior KeyValueStore follows when the maxmemory limit is reached." placeholder:"allkeys-lru"`
	AllowedCidrs    []storage.IPv4CIDR                   `help:"AllowedCIDRs specify the allowed IP addresses, connecting to the instance." placeholder:"0.0.0.0/0"`
	Wait            bool                                 `default:"true" help:"Wait until KeyValueStore is created."`
	WaitTimeout     time.Duration                        `default:"600s" help:"Duration to wait for KeyValueStore getting ready. Only relevant if --wait is set."`
}

func (cmd *keyValueStoreCmd) Run(ctx context.Context, client *api.Client) error {
	keyValueStore, err := cmd.newKeyValueStore(client.Project)
	if err != nil {
		return err
	}

	c := newCreator(client, keyValueStore, "keyvaluestore")
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !cmd.Wait {
		return nil
	}

	return c.wait(ctx, waitStage{
		objectList: &storage.KeyValueStoreList{},
		onResult: func(event watch.Event) (bool, error) {
			if c, ok := event.Object.(*storage.KeyValueStore); ok {
				return isAvailable(c), nil
			}
			return false, nil
		},
	},
	)
}

func (cmd *keyValueStoreCmd) newKeyValueStore(namespace string) (*storage.KeyValueStore, error) {
	name := getName(cmd.Name)

	keyValueStore := &storage.KeyValueStore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: storage.KeyValueStoreSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      "keyvaluestore-" + name,
					Namespace: namespace,
				},
			},
			ForProvider: storage.KeyValueStoreParameters{
				Location:        meta.LocationName(cmd.Location),
				MaxMemoryPolicy: cmd.MaxMemoryPolicy,
				AllowedCIDRs:    cmd.AllowedCidrs,
			},
		},
	}

	if cmd.MemorySize != "" {
		q, err := resource.ParseQuantity(cmd.MemorySize)
		if err != nil {
			return keyValueStore, fmt.Errorf("error parsing memory size %q: %w", cmd.MemorySize, err)
		}

		keyValueStore.Spec.ForProvider.MemorySize = &storage.KeyValueStoreMemorySize{Quantity: q}
	}

	return keyValueStore, nil
}
