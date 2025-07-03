package get

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
)

type mysqlDatabaseCmd struct {
	databaseCmd
	PrintCharacterSet bool `help:"Print the character set of the MySQL database. Requires name to be set." xor:"print"`
}

func (cmd *mysqlDatabaseCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	databaseList := &storage.MySQLDatabaseList{}
	databaseResources := make([]resource.Managed, 0)

	if err := get.list(ctx, client, databaseList, api.MatchName(cmd.Name)); err != nil {
		return err
	}

	for i := range databaseList.Items {
		databaseResources = append(databaseResources, &databaseList.Items[i])
	}

	if cmd.Name != "" && cmd.PrintCharacterSet {
		return cmd.printMySQLCharacterSet(databaseResources[0].(*storage.MySQLDatabase))
	}

	return cmd.runDatabaseCmd(ctx, client, get,
		databaseList, databaseResources, storage.MySQLDatabaseKind,
		func(ctx context.Context, client *api.Client, mg resource.Managed) error {
			return cmd.printConnectionString(ctx, client, mg.(*storage.MySQLDatabase))
		},
		func(res []resource.Managed, get *Cmd, header bool) error {
			dbs := make([]storage.MySQLDatabase, len(res))
			for i, r := range res {
				dbs[i] = *r.(*storage.MySQLDatabase)
			}

			return cmd.printMySQLDatabases(dbs, get, header)
		},
	)
}

func (cmd *mysqlDatabaseCmd) printMySQLDatabases(list []storage.MySQLDatabase, get *Cmd, header bool) error {
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

// printConnectionString according to the MySQL documentation:
// https://dev.mysql.com/doc/refman/8.4/en/connecting-using-uri-or-key-value-pairs.html#connecting-using-uri
func (cmd *mysqlDatabaseCmd) printConnectionString(ctx context.Context, client *api.Client, mdb *storage.MySQLDatabase) error {
	secrets, err := getConnectionSecretMap(ctx, client, mdb)
	if err != nil {
		return err
	}

	for db, pw := range secrets {
		fmt.Fprintf(cmd.out, "mysql://%s:%s@%s/%s\n",
			db,
			pw,
			mdb.Status.AtProvider.FQDN,
			db,
		)
		break
	}

	return nil
}

func (cmd *mysqlDatabaseCmd) printMySQLCharacterSet(mdb *storage.MySQLDatabase) error {
	fmt.Fprintln(defaultOut(cmd.out), mdb.Spec.ForProvider.CharacterSet.Name)

	return nil
}
