package delete

import (
	"context"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
)

type mysqlDatabaseCmd struct {
	resourceCmd
}

func (cmd *mysqlDatabaseCmd) Run(ctx context.Context, client *api.Client) error {
	mysqlDatabase := &storage.MySQLDatabase{}
	mysqlDatabase.SetName(cmd.Name)
	mysqlDatabase.SetNamespace(client.Project)

	return newDeleter(mysqlDatabase, storage.MySQLDatabaseKind).
		deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force)
}
