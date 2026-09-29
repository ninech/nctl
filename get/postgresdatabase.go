package get

import (
	"context"
	"fmt"
	"strconv"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type postgresDatabaseCmd struct{ ServiceCmd }

func (cmd *postgresDatabaseCmd) Run(ctx context.Context, c *api.Client, get *Cmd) error {
	return get.listPrint(ctx, c, cmd, api.MatchName(cmd.Name))
}

func (cmd *postgresDatabaseCmd) list() client.ObjectList {
	return &storage.PostgresDatabaseList{}
}

func (cmd *postgresDatabaseCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	databaseList, ok := list.(*storage.PostgresDatabaseList)
	if !ok {
		return fmt.Errorf("expected %T, got %T", &storage.PostgresDatabaseList{}, list)
	}

	return cmd.run(ctx, client, out, databaseList, service{
		kind:             storage.PostgresDatabaseKind,
		connectionString: cmd.connectionString,
		printList:        cmd.printPostgresDatabases,
		caCert: func(mg resource.Managed) (string, error) {
			db, ok := mg.(*storage.PostgresDatabase)
			if !ok {
				return "", fmt.Errorf("expected %T, got %T", &storage.PostgresDatabase{}, mg)
			}
			return db.Status.AtProvider.CACert, nil
		},
	})
}

func (cmd *postgresDatabaseCmd) printPostgresDatabases(resources resource.ManagedList, out *output, header bool) error {
	dbs, ok := resources.(*storage.PostgresDatabaseList)
	if !ok {
		return fmt.Errorf("expected %T, got %T", &storage.PostgresDatabaseList{}, dbs)
	}

	if header {
		out.writeHeader("NAME", "LOCATION", "VERSION", "FQDN", "SIZE", "CONNECTIONS")
	}

	for _, db := range dbs.Items {
		out.writeTabRow(db.Namespace, db.Name, string(db.Spec.ForProvider.Location), string(db.Spec.ForProvider.Version), db.Status.AtProvider.FQDN, db.Status.AtProvider.Size.String(), strconv.FormatUint(uint64(db.Status.AtProvider.Connections), 10))
	}

	return out.tabWriter.Flush()
}

func (cmd *postgresDatabaseCmd) connectionString(mg resource.Managed, user, password string) (string, error) {
	my, ok := mg.(*storage.PostgresDatabase)
	if !ok {
		return "", fmt.Errorf("expected %T, got %T", &storage.PostgresDatabase{}, mg)
	}

	return postgresConnectionString(my.Status.AtProvider.FQDN, user, password, user)
}
