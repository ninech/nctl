package delete

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
)

type postgresCmd struct {
	resourceCmd
}

func (cmd *postgresCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	postgres := &storage.Postgres{}
	postgresName := types.NamespacedName{Name: cmd.Name, Namespace: client.Project}
	if err := client.Get(ctx, postgresName, postgres); err != nil {
		return fmt.Errorf("unable to get postgres %q: %w", postgres.Name, err)
	}

	return newDeleter(postgres, storage.PostgresKind).deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force)
}
