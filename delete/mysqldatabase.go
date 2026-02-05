package delete

import (
	"context"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mysqlDatabaseCmd struct {
	resourceCmd
}

func (cmd *mysqlDatabaseCmd) Run(ctx context.Context, client *api.Client) error {
	mysqlDatabase := &storage.MySQLDatabase{ObjectMeta: metav1.ObjectMeta{Name: cmd.Name, Namespace: client.Project}}
	return cmd.newDeleter(mysqlDatabase, storage.MySQLDatabaseKind).
		deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force)
}
