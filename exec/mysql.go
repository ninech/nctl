package exec

import (
	"context"
	"fmt"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	mysqlPort    = "3306"
	mysqlCommand = "mysql"
)

type mysqlCmd struct {
	serviceCmd
	Database string `name:"database" short:"d" completion-predictor:"mysql_databases" help:"Database name to connect to."`
}

// Help displays usage examples for the mysql exec command.
func (cmd mysqlCmd) Help() string {
	return `Examples:
  # Connect to a MySQL instance interactively
  nctl exec mysql myinstance

  # Connect to a specific database
  nctl exec mysql myinstance -d mydb

  # Import a SQL dump via pipe
  cat dump.sql | nctl exec mysql myinstance

  # Pass extra flags to mysql (after --)
  nctl exec mysql myinstance -- --batch
`
}

func (cmd *mysqlCmd) Run(ctx context.Context, client *api.Client) error {
	my := &storage.MySQL{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}
	if err := client.Get(ctx, client.Name(cmd.Name), my); err != nil {
		return fmt.Errorf("getting mysql %q: %w", cmd.Name, err)
	}
	return connectAndExec(ctx, client, my, mysqlConnector{database: cmd.Database}, cmd.serviceCmd)
}

// mysqlConnector implements cmdExecutor for storage.MySQL instances.
type mysqlConnector struct {
	database string
}

func (mysqlConnector) Command() string { return mysqlCommand }

func (mysqlConnector) Endpoint(my *storage.MySQL) string {
	if my.Status.AtProvider.FQDN == "" {
		return ""
	}
	return my.Status.AtProvider.FQDN + ":" + mysqlPort
}

func (mysqlConnector) AllowedCIDRs(my *storage.MySQL) []meta.IPv4CIDR {
	return my.Spec.ForProvider.AllowedCIDRs
}

func (mysqlConnector) Update(ctx context.Context, client *api.Client, my *storage.MySQL, cidrs []meta.IPv4CIDR) error {
	current := &storage.MySQL{}
	if err := client.Get(ctx, api.ObjectName(my), current); err != nil {
		return err
	}
	current.Spec.ForProvider.AllowedCIDRs = cidrs
	return client.Update(ctx, current)
}

func (c mysqlConnector) Args(my *storage.MySQL, user, pw string) ([]string, func(), error) {
	return mysqlArgs(my.Status.AtProvider.FQDN, c.database, my.Status.AtProvider.CACert, user, pw)
}

// mysqlArgs returns the mysql CLI arguments for connecting to a MySQL instance.
// dbName is appended as a positional argument when non-empty.
func mysqlArgs(fqdn, dbName, caCertBase64, user, pw string) ([]string, func(), error) {
	caPath, cleanup, err := writeCACert(caCertBase64)
	if err != nil {
		return nil, func() {}, err
	}

	args := []string{
		"-h", fqdn,
		"-P", mysqlPort,
		"-u", user,
		"-p" + pw,
		"--ssl-mode=REQUIRED",
	}
	if caPath != "" {
		args = append(args, "--ssl-ca="+caPath)
	}
	if dbName != "" {
		args = append(args, dbName)
	}

	return args, cleanup, nil
}
