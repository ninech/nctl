package exec

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

// NewCmd builds the mysql command. Credentials are passed via --defaults-extra-file
// rather than -u/-p flags so they do not appear in the process argument list.
func (c mysqlConnector) NewCmd(ctx context.Context, my *storage.MySQL, user, pw string) (*exec.Cmd, func(), error) {
	return newMySQLCmd(ctx, my.Status.AtProvider.FQDN, c.database, my.Status.AtProvider.CACert, user, pw)
}

// newMySQLCmd returns an exec.Cmd for mysql with credentials in a temp options file.
// When a CA cert is provided the connection uses VERIFY_CA, otherwise REQUIRED.
func newMySQLCmd(ctx context.Context, fqdn, dbName, caCertBase64, user, pw string) (*exec.Cmd, func(), error) {
	dir, cleanup, err := createTempDir()
	if err != nil {
		return nil, func() {}, err
	}

	caPath, err := writeCACert(dir, caCertBase64)
	if err != nil {
		cleanup()
		return nil, func() {}, err
	}

	cfgPath, err := writeMySQLConfig(dir, user, pw)
	if err != nil {
		cleanup()
		return nil, func() {}, err
	}

	// --defaults-extra-file must precede all other options.
	args := []string{
		"--defaults-extra-file=" + cfgPath,
		"-h", fqdn,
		"-P", mysqlPort,
	}
	if caPath != "" {
		args = append(args, "--ssl-ca="+caPath, "--ssl-mode=VERIFY_CA")
	} else {
		args = append(args, "--ssl-mode=REQUIRED")
	}
	if dbName != "" {
		args = append(args, dbName)
	}

	return exec.CommandContext(ctx, mysqlCommand, args...), cleanup, nil
}

// writeMySQLConfig writes a temporary MySQL options file into dir containing
// the given credentials. The file is mode 0600 so other local users cannot read it.
func writeMySQLConfig(dir, user, pw string) (string, error) {
	if dir == "" {
		return "", nil
	}

	path := filepath.Join(dir, "my.cnf")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("creating MySQL config temp file %q: %w", path, err)
	}
	defer f.Close()

	if _, err = fmt.Fprintf(f, "[client]\nuser=%s\npassword=%s\n", mysqlConfigEscape(user), mysqlConfigEscape(pw)); err != nil {
		return "", fmt.Errorf("writing MySQL config %q: %w", path, err)
	}

	return f.Name(), nil
}

// mysqlConfigEscape escapes a value for use in a MySQL option file.
// Values are double-quoted; internal double quotes and backslashes are escaped.
func mysqlConfigEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}
