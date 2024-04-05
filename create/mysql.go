package create

import (
	"context"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
)

type mySQLCmd struct {
	Name                  string                                 `arg:"" default:"" help:"Name of the MySQL instance. A random name is generated if omitted."`
	Location              string                                 `default:"nine-cz41" help:"Location where the MySQL instance is created."`
	MachineType           infra.MachineType                      `help:"Defines the sizing for a particular MySQL instance." placeholder:"nine-standard-1" default:"nine-standard-1"`
	AllowedCidrs          []storage.IPv4CIDR                     `help:"Specify the allowed IP addresses, connecting to the instance." placeholder:"0.0.0.0/0"`
	SSHKeys               []storage.SSHKey                       `help:"Contains a list of SSH public keys, allowed to connect to the db server, in order to up-/download and directly restore database backups."`
	SQLMode               *[]storage.MySQLMode                   `help:"Configures the sql_mode setting. Modes affect the SQL syntax MySQL supports and the data validation checks it performs."`
	CharacterSetName      string                                 `help:"Configures the character_set_server variable."`
	CharacterSetCollation string                                 `help:"Configures the collation_server variable."`
	LongQueryTime         storage.LongQueryTime                  `help:"Configures the long_query_time variable. If a query takes longer than this many seconds, the the query is logged to the slow query log file."`
	MinWordLength         *int                                   `help:"Configures the ft_min_word_len and innodb_ft_min_token_size variables."`
	TransactionIsolation  storage.MySQLTransactionCharacteristic `help:"Configures the transaction_isolation variable."`
	KeepDailyBackups      *int                                   `help:"Number of daily database backups to keep. Note that setting this to 0, backup will be disabled and existing dumps deleted immediately."`
	Wait                  bool                                   `default:"true" help:"Wait until MySQL instance is created."`
	WaitTimeout           time.Duration                          `default:"900s" help:"Duration to wait for MySQL getting ready. Only relevant if --wait is set."`
}

func (cmd mySQLCmd) Run(ctx context.Context, client *api.Client) error {
	mysql, err := cmd.newMySQL(client.Project)
	if err != nil {
		return err
	}

	c := newCreator(client, mysql, "mysql")
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !cmd.Wait {
		return nil
	}

	return c.wait(ctx, waitStage{
		objectList: &storage.MySQLList{},
		onResult: func(event watch.Event) (bool, error) {
			if c, ok := event.Object.(*storage.MySQL); ok {
				return isAvailable(c), nil
			}
			return false, nil
		},
	},
	)
}

func (cmd *mySQLCmd) newMySQL(namespace string) (*storage.MySQL, error) {
	name := getName(cmd.Name)

	mySQL := &storage.MySQL{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: storage.MySQLSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      "mysql-" + name,
					Namespace: namespace,
				},
			},
			ForProvider: storage.MySQLParameters{
				Location:     meta.LocationName(cmd.Location),
				MachineType:  cmd.MachineType,
				AllowedCIDRs: []storage.IPv4CIDR{},
				SSHKeys:      []storage.SSHKey{},
				SQLMode:      cmd.SQLMode,
				CharacterSet: storage.MySQLCharacterSet{
					Name:      cmd.CharacterSetName,
					Collation: cmd.CharacterSetCollation,
				},
				LongQueryTime:        cmd.LongQueryTime,
				MinWordLength:        cmd.MinWordLength,
				TransactionIsolation: cmd.TransactionIsolation,
				KeepDailyBackups:     cmd.KeepDailyBackups,
			},
		},
	}

	if cmd.AllowedCidrs != nil {
		mySQL.Spec.ForProvider.AllowedCIDRs = cmd.AllowedCidrs
	}
	if cmd.SSHKeys != nil {
		mySQL.Spec.ForProvider.SSHKeys = cmd.SSHKeys
	}

	return mySQL, nil
}
