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

type mysqlDatabaseCmd struct{ databaseCmd }

func (cmd *mysqlDatabaseCmd) Run(ctx context.Context, c *api.Client, get *Cmd) error {
	return get.listPrint(ctx, c, cmd, api.MatchName(cmd.Name))
}

func (cmd *mysqlDatabaseCmd) list() client.ObjectList {
	return &storage.MySQLDatabaseList{}
}

func (cmd *mysqlDatabaseCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	databaseList, ok := list.(*storage.MySQLDatabaseList)
	if !ok {
		return fmt.Errorf("expected %T, got %T", &storage.MySQLDatabaseList{}, list)
	}

	return cmd.run(ctx, client, &Cmd{output: *out},
		databaseList, storage.MySQLDatabaseKind,
		cmd.printConnectionString,
		cmd.printMySQLDatabases,
		func(mg resource.Managed) (string, error) {
			db, ok := mg.(*storage.MySQLDatabase)
			if !ok {
				return "", fmt.Errorf("expected %T, got %T", &storage.MySQLDatabase{}, mg)
			}

			return db.Status.AtProvider.CACert, nil
		},
	)
}

func (cmd *mysqlDatabaseCmd) printMySQLDatabases(databases []resource.Managed, get *Cmd, header bool) error {
	if header {
		get.writeHeader("NAME", "FQDN", "LOCATION", "COLLATION", "SIZE", "CONNECTIONS")
	}

	for _, mg := range databases {
		db, ok := mg.(*storage.MySQLDatabase)
		if !ok {
			return fmt.Errorf("expected %T, got %T", &storage.MySQLDatabase{}, mg)
		}

		get.writeTabRow(
			db.Namespace,
			db.Name,
			db.Status.AtProvider.FQDN,
			string(db.Spec.ForProvider.Location),
			db.Spec.ForProvider.CharacterSet.Collation,
			db.Status.AtProvider.Size.String(),
			strconv.FormatUint(uint64(db.Status.AtProvider.Connections), 10),
		)
	}

	return get.tabWriter.Flush()
}

func (cmd *mysqlDatabaseCmd) printConnectionString(ctx context.Context, client *api.Client, mg resource.Managed) error {
	my, ok := mg.(*storage.MySQLDatabase)
	if !ok {
		return fmt.Errorf("expected mysqldatabase, got %T", mg)
	}

	secrets, err := getConnectionSecretMap(ctx, client, my)
	if err != nil {
		return err
	}

	for db, pw := range secrets {
		fmt.Fprintln(cmd.out, mySQLConnectionString(my.Status.AtProvider.FQDN, db, db, pw))
		return nil
	}

	return nil
}

// mySQLConnectionString according to the MySQL documentation:
// https://dev.mysql.com/doc/refman/8.4/en/connecting-using-uri-or-key-value-pairs.html#connecting-using-uri
func mySQLConnectionString(fqdn, user, db string, pw []byte) string {
	u := &url.URL{
		Scheme: "mysql",
		Host:   fqdn,
		User:   url.UserPassword(user, string(pw)),
		Path:   db,
	}

	return u.String()
}
