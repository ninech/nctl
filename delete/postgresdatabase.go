package delete

import (
	"context"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
)

type postgresDatabaseCmd struct {
	resourceCmd
}

func (cmd *postgresDatabaseCmd) Run(ctx context.Context, client *api.Client) error {
	postgresDatabase := &storage.PostgresDatabase{}
	postgresDatabase.SetName(cmd.Name)
	postgresDatabase.SetNamespace(client.Project)

	return newDeleter(postgresDatabase, storage.PostgresDatabaseKind).
		deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force)
}
