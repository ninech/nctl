package delete

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
)

type keyValueStoreCmd struct {
	Name        string        `arg:"" help:"Name of the KeyValueStore resource."`
	Force       bool          `default:"false" help:"Do not ask for confirmation of deletion."`
	Wait        bool          `default:"true" help:"Wait until KeyValueStore is fully deleted."`
	WaitTimeout time.Duration `default:"300s" help:"Duration to wait for the deletion. Only relevant if wait is set."`
}

func (cmd *keyValueStoreCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	keyValueStore := &storage.KeyValueStore{}
	keyValueStoreName := types.NamespacedName{Name: cmd.Name, Namespace: client.Project}
	if err := client.Get(ctx, keyValueStoreName, keyValueStore); err != nil {
		return fmt.Errorf("unable to get keyvaluestore %q: %w", keyValueStore.Name, err)
	}

	return newDeleter(keyValueStore, storage.KeyValueStoreKind).deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force)
}
