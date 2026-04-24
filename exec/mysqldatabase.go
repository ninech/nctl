package exec

import (
	"context"
	"fmt"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mysqlDatabaseCmd struct {
	serviceCmd
}

// Help displays usage examples for the mysqldatabase exec command.
func (cmd mysqlDatabaseCmd) Help() string {
	return `Examples:
  # Connect to a MySQL database interactively
  nctl exec mysqldatabase mydb

  # Import a SQL dump via pipe
  cat dump.sql | nctl exec mysqldatabase mydb
`
}

// Run connects to the named MySQLDatabase resource.
func (cmd *mysqlDatabaseCmd) Run(ctx context.Context, client *api.Client) error {
	db := &storage.MySQLDatabase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}
	if err := client.Get(ctx, client.Name(cmd.Name), db); err != nil {
		return fmt.Errorf("getting mysqldatabase %q: %w", cmd.Name, err)
	}
	return connectAndExec(ctx, client, db, mysqlDatabaseConnector{}, cmd.serviceCmd)
}

// mysqlDatabaseConnector implements cmdExecutor for storage.MySQLDatabase resources.
// It does not implement accessManager because the parent MySQL instance manages CIDRs.
type mysqlDatabaseConnector struct{}

// Command returns the CLI binary name for connecting to a MySQL database.
func (mysqlDatabaseConnector) Command() string { return mysqlCommand }

// Endpoint returns the host:port for the TCP connectivity check.
func (mysqlDatabaseConnector) Endpoint(db *storage.MySQLDatabase) string {
	if db.Status.AtProvider.FQDN == "" {
		return ""
	}
	return db.Status.AtProvider.FQDN + ":" + mysqlPort
}

// Args returns the mysql CLI arguments for connecting to a MySQLDatabase.
// dbName is appended as a positional argument when non-empty.
func (mysqlDatabaseConnector) Args(db *storage.MySQLDatabase, user, pw string) ([]string, func(), error) {
	dbName := db.Status.AtProvider.Name
	if dbName == "" {
		dbName = user
	}
	return mysqlArgs(db.Status.AtProvider.FQDN, dbName, db.Status.AtProvider.CACert, user, pw)
}
