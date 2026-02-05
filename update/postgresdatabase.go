package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type postgresDatabaseCmd struct {
	resourceCmd
}

func (cmd *postgresDatabaseCmd) Run(ctx context.Context, client *api.Client) error {
	postgresDatabase := &storage.PostgresDatabase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}

	upd := cmd.newUpdater(client, postgresDatabase, storage.PostgresDatabaseKind, func(current resource.Managed) error {
		postgresDatabase, ok := current.(*storage.PostgresDatabase)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, storage.PostgresDatabase{})
		}

		cmd.applyUpdates(postgresDatabase)
		return nil
	})

	return upd.Update(ctx)
}

func (cmd *postgresDatabaseCmd) applyUpdates(_ *storage.PostgresDatabase) {
	cmd.Warningf("there are no attributes for postgresdatabase which can be updated after creation. Applying update without any changes.")
}
