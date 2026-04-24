package exec

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os/exec"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
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

// NewCmd builds the psql command with PGPASSWORD passed via env instead of the connection URL.
func (c postgresConnector) NewCmd(ctx context.Context, pg *storage.Postgres, user, pw string) (*exec.Cmd, func(), error) {
	return newPsqlCmd(ctx, pg.Status.AtProvider.FQDN, c.database, pg.Status.AtProvider.CACert, user, pw)
}

// newPsqlCmd returns an exec.Cmd for psql. The password is passed via PGPASSWORD
// rather than the connection URL so it does not appear in the process argument list.
func newPsqlCmd(ctx context.Context, fqdn, dbName, caCertBase64, user, pw string) (*exec.Cmd, func(), error) {
	dir, cleanup, err := createTempDir()
	if err != nil {
		return nil, func() {}, err
	}

	caPath, err := writeCACert(dir, caCertBase64)
	if err != nil {
		cleanup()
		return nil, func() {}, err
	}

	connURL := postgresConnectionURL(fqdn, user, dbName, caPath)
	cmd := exec.CommandContext(ctx, postgresCommand, connURL.String())
	cmd.Env = []string{"PGPASSWORD=" + pw}
	return cmd, cleanup, nil
}

// postgresConnectionURL builds a psql connection URL without a password.
// sslmode is set to verify-ca when a CA cert path is provided, otherwise require.
func postgresConnectionURL(fqdn, user, db, caPath string) *url.URL {
	if db == "" {
		db = user
	}
	q := url.Values{}
	if caPath != "" {
		q.Set("sslmode", "verify-ca")
		q.Set("sslrootcert", caPath)
	} else {
		q.Set("sslmode", "require")
	}
	return &url.URL{
		Scheme:   "postgres",
		Host:     fqdn,
		User:     url.User(user),
		Path:     db,
		RawQuery: q.Encode(),
	}
}
