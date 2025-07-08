package get

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
)

type postgresDatabaseCmd struct {
	databaseCmd
}

func (cmd *postgresDatabaseCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	databaseList := &storage.PostgresDatabaseList{}
	databaseResources := make([]resource.Managed, 0)

	if err := get.list(ctx, client, databaseList, api.MatchName(cmd.Name)); err != nil {
		return err
	}

	for i := range databaseList.Items {
		databaseResources = append(databaseResources, &databaseList.Items[i])
	}

	return cmd.runDatabaseCmd(ctx, client, get,
		databaseList, databaseResources, storage.PostgresDatabaseKind,
		func(ctx context.Context, client *api.Client, mg resource.Managed) error {
			return cmd.printConnectionString(ctx, client, mg.(*storage.PostgresDatabase))
		},
		func(res []resource.Managed, get *Cmd, header bool) error {
			dbs := make([]storage.PostgresDatabase, len(res))
			for i, r := range res {
				dbs[i] = *r.(*storage.PostgresDatabase)
			}

			return cmd.printPostgresDatabases(dbs, get, header)
		},
	)
}

func (cmd *postgresDatabaseCmd) printPostgresDatabases(list []storage.PostgresDatabase, get *Cmd, header bool) error {
	databases := make([]Database, len(list))
	for i, db := range list {
		databases[i] = Database{
			Namespace:   db.Namespace,
			Name:        db.Name,
			FQDN:        db.Status.AtProvider.FQDN,
			Location:    string(db.Spec.ForProvider.Location),
			Size:        db.Status.AtProvider.Size.String(),
			Connections: fmt.Sprintf("%d", db.Status.AtProvider.Connections),
		}
	}

	return printDatabases(cmd.out, get, databases, header)
}

// printConnectionString according to the PostgreSQL documentation:
// https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING
func (cmd *postgresDatabaseCmd) printConnectionString(ctx context.Context, client *api.Client, pgd *storage.PostgresDatabase) error {
	secrets, err := getConnectionSecretMap(ctx, client, pgd)
	if err != nil {
		return err
	}

	for db, pw := range secrets {
		fmt.Fprintf(cmd.out, "postgres://%s:%s@%s/%s\n",
			db,
			pw,
			pgd.Status.AtProvider.FQDN,
			db,
		)
		break
	}

	return nil
}
