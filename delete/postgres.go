package delete

import (
	"context"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type postgresCmd struct {
	resourceCmd
}

func (cmd *postgresCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	postgres := &storage.Postgres{ObjectMeta: metav1.ObjectMeta{Name: cmd.Name, Namespace: client.Project}}
	return cmd.newDeleter(postgres, storage.PostgresKind).deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force)
}
