package exec

import (
	"context"
	"fmt"
	"net"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type postgresDatabaseCmd struct {
	serviceCmd
}

// Help displays usage examples for the postgresdatabase exec command.
func (cmd postgresDatabaseCmd) Help() string {
	return `Examples:
  # Connect to a PostgreSQL database interactively
  nctl exec postgresdatabase mydb

  # Import a SQL dump via pipe
  cat dump.sql | nctl exec postgresdatabase mydb
`
}

// Run connects to the named PostgresDatabase resource.
func (cmd *postgresDatabaseCmd) Run(ctx context.Context, client *api.Client) error {
	db := &storage.PostgresDatabase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}
	if err := client.Get(ctx, client.Name(cmd.Name), db); err != nil {
		return fmt.Errorf("getting postgresdatabase %q: %w", cmd.Name, err)
	}
	return connectAndExec(ctx, client, db, postgresDatabaseConnector{}, cmd.serviceCmd)
}

// postgresDatabaseConnector implements cmdExecutor for storage.PostgresDatabase resources.
// It does not implement accessManager because the parent Postgres instance manages CIDRs.
type postgresDatabaseConnector struct{}

// Command returns the CLI binary name for connecting to a PostgreSQL database.
func (postgresDatabaseConnector) Command() string { return postgresCommand }

// Endpoint returns the host:port for the TCP connectivity check.
func (postgresDatabaseConnector) Endpoint(db *storage.PostgresDatabase) string {
	if db.Status.AtProvider.FQDN == "" {
		return ""
	}
	return net.JoinHostPort(db.Status.AtProvider.FQDN, postgresPort)
}

// Args returns the psql CLI arguments for connecting to a PostgresDatabase.
func (postgresDatabaseConnector) Args(db *storage.PostgresDatabase, user, pw string) ([]string, func(), error) {
	return psqlArgs(db.Status.AtProvider.FQDN, db.Status.AtProvider.Name, db.Status.AtProvider.CACert, user, pw)
}
