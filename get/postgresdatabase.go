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
		cmd.connectionString,
		cmd.printPostgresDatabases,
		func(mg resource.Managed) (string, error) {
			db, ok := mg.(*storage.PostgresDatabase)
			if !ok {
				return "", fmt.Errorf("expected %T, got %T", &storage.PostgresDatabase{}, mg)
			}
			return db.Status.AtProvider.CACert, nil
		},
	)
}

func (cmd *postgresDatabaseCmd) printPostgresDatabases(resources resource.ManagedList, get *Cmd, header bool) error {
	dbs, ok := resources.(*storage.PostgresDatabaseList)
	if !ok {
		return fmt.Errorf("expected %T, got %T", &storage.PostgresDatabaseList{}, dbs)
	}

	if header {
		get.writeHeader("NAME", "LOCATION", "VERSION", "FQDN", "SIZE", "CONNECTIONS")
	}

	for _, db := range dbs.Items {
		get.writeTabRow(db.Namespace, db.Name, string(db.Spec.ForProvider.Location), string(db.Spec.ForProvider.Version), db.Status.AtProvider.FQDN, db.Status.AtProvider.Size.String(), strconv.FormatUint(uint64(db.Status.AtProvider.Connections), 10))
	}

	return get.tabWriter.Flush()
}

func (cmd *postgresDatabaseCmd) connectionString(mg resource.Managed, secrets map[string][]byte) (string, error) {
	my, ok := mg.(*storage.PostgresDatabase)
	if !ok {
		return "", fmt.Errorf("expected %T, got %T", &storage.PostgresDatabase{}, mg)
	}

	for user, pw := range secrets {
		return postgresConnectionString(my.Status.AtProvider.FQDN, user, user, pw), nil
	}

	return "", nil
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
