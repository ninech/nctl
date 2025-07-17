package delete

import (
	"context"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type postgresDatabaseCmd struct {
	resourceCmd
}

func (cmd *postgresDatabaseCmd) Run(ctx context.Context, client *api.Client) error {
	postgresDatabase := &storage.PostgresDatabase{ObjectMeta: metav1.ObjectMeta{Name: cmd.Name, Namespace: client.Project}}
	return newDeleter(postgresDatabase, storage.PostgresDatabaseKind).deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force)
}
