package get

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type postgresDatabaseCmd struct{ databaseCmd }

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

	return cmd.run(ctx, client, &Cmd{output: *out},
		databaseList, storage.PostgresDatabaseKind,
		cmd.printConnectionString,
		cmd.printPostgresDatabases,
		func(mg resource.Managed) (string, error) {
			db, ok := mg.(*storage.PostgresDatabase)
			if !ok {
				return "", fmt.Errorf("expected postgresdatabase, got %T", mg)
			}
			return db.Status.AtProvider.CACert, nil
		},
	)
}

func (cmd *postgresDatabaseCmd) printPostgresDatabases(resources []resource.Managed, get *Cmd, header bool) error {
	if header {
		get.writeHeader("NAME", "FQDN", "LOCATION", "SIZE", "CONNECTIONS")
	}

	for _, mg := range resources {
		db, ok := mg.(*storage.PostgresDatabase)
		if !ok {
			return fmt.Errorf("expected postgresdatabase, got %T", mg)
		}

		get.writeTabRow(
			db.Namespace,
			db.Name,
			db.Status.AtProvider.FQDN,
			string(db.Spec.ForProvider.Location),
			db.Status.AtProvider.Size.String(),
			strconv.FormatUint(uint64(db.Status.AtProvider.Connections), 10),
		)
	}

	return get.tabWriter.Flush()
}

func (cmd *postgresDatabaseCmd) printConnectionString(ctx context.Context, client *api.Client, mg resource.Managed) error {
	pg, ok := mg.(*storage.PostgresDatabase)
	if !ok {
		return fmt.Errorf("expected postgresdatabase, got %T", mg)
	}

	secrets, err := getConnectionSecretMap(ctx, client, pg)
	if err != nil {
		return err
	}

	for db, pw := range secrets {
		fmt.Fprintln(cmd.out, postgresConnectionString(pg.Status.AtProvider.FQDN, db, db, pw))
		return nil
	}

	return nil
}

// postgresConnectionString according to the PostgreSQL documentation:
// https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING
func postgresConnectionString(fqdn, user, db string, pw []byte) string {
	u := &url.URL{
		Scheme: "postgres",
		Host:   fqdn,
		User:   url.UserPassword(user, string(pw)),
		Path:   db,
	}

	return u.String()
}
