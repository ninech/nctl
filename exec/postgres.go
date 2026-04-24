package exec

import (
	"context"
	"fmt"
	"net"
	"net/url"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/get"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	postgresPort    = "5432"
	postgresCommand = "psql"
)

type postgresCmd struct {
	serviceCmd
	Database string `name:"database" short:"d" default:"postgres" completion-predictor:"postgres_databases" help:"Database name to connect to."`
}

// Help displays usage examples for the postgres exec command.
func (cmd postgresCmd) Help() string {
	return `Examples:
  # Connect to a PostgreSQL instance interactively
  nctl exec postgres myinstance

  # Connect to a specific database
  nctl exec postgres myinstance -d mydb

  # Import a SQL dump via pipe
  cat dump.sql | nctl exec postgres myinstance

  # Pass extra flags to psql (after --)
  nctl exec postgres myinstance -- --no-pager
`
}

func (cmd *postgresCmd) Run(ctx context.Context, client *api.Client) error {
	pg := &storage.Postgres{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}
	if err := client.Get(ctx, client.Name(cmd.Name), pg); err != nil {
		return fmt.Errorf("getting postgres %q: %w", cmd.Name, err)
	}
	return connectAndExec(ctx, client, pg, postgresConnector{database: cmd.Database}, cmd.serviceCmd)
}

// postgresConnector implements cmdExecutor for storage.Postgres instances.
type postgresConnector struct {
	database string
}

func (postgresConnector) Command() string { return postgresCommand }

func (postgresConnector) Endpoint(pg *storage.Postgres) string {
	if pg.Status.AtProvider.FQDN == "" {
		return ""
	}
	return net.JoinHostPort(pg.Status.AtProvider.FQDN, postgresPort)
}

func (postgresConnector) AllowedCIDRs(pg *storage.Postgres) []meta.IPv4CIDR {
	return pg.Spec.ForProvider.AllowedCIDRs
}

func (postgresConnector) Update(ctx context.Context, client *api.Client, pg *storage.Postgres, cidrs []meta.IPv4CIDR) error {
	current := &storage.Postgres{}
	if err := client.Get(ctx, api.ObjectName(pg), current); err != nil {
		return err
	}
	current.Spec.ForProvider.AllowedCIDRs = cidrs
	return client.Update(ctx, current)
}

func (c postgresConnector) Args(pg *storage.Postgres, user, pw string) ([]string, func(), error) {
	return psqlArgs(pg.Status.AtProvider.FQDN, c.database, pg.Status.AtProvider.CACert, user, pw)
}

// psqlArgs returns the psql arguments for a given database.
func psqlArgs(fqdn, name, caCertBase64, user, pw string) ([]string, func(), error) {
	caPath, cleanup, err := writeCACert(caCertBase64)
	if err != nil {
		return nil, func() {}, err
	}

	dbName := name
	if dbName == "" {
		dbName = user
	}

	conn := postgresConnectionStringCA(fqdn, user, dbName, []byte(pw), caPath)
	return []string{conn.String()}, cleanup, nil
}

// postgresConnectionStringCA returns a PostgreSQL connection string with CA certificate verification enabled.
func postgresConnectionStringCA(fqdn string, user string, db string, pw []byte, caPath string) *url.URL {
	conn := get.PostgresConnectionString(fqdn, user, db, pw)
	q := conn.Query()
	q.Set("sslrootcert", caPath)
	q.Set("sslmode", "verify-ca")
	conn.RawQuery = q.Encode()

	return conn
}
