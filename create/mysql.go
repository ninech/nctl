package create

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/alecthomas/kong"
	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/file"
)

type mySQLCmd struct {
	resourceCmd
	Location              string                                 `placeholder:"${mysql_location_default}" help:"Location where the MySQL instance is created. Available locations are: ${mysql_location_options}"`
	MachineType           string                                 `placeholder:"${mysql_machine_default}" help:"Defines the sizing for a particular MySQL instance. Available types: ${mysql_machine_types}"`
	AllowedCidrs          []meta.IPv4CIDR                        `placeholder:"203.0.113.1/32" help:"Specifies the IP addresses allowed to connect to the instance." `
	SSHKeys               []storage.SSHKey                       `help:"Contains a list of SSH public keys, allowed to connect to the db server, in order to up-/download and directly restore database backups."`
	SSHKeysFile           string                                 `help:"Path to a file containing a list of SSH public keys (see above), separated by newlines."`
	SQLMode               *[]storage.MySQLMode                   `placeholder:"\"MODE1, MODE2, ...\"" help:"Configures the sql_mode setting. Modes affect the SQL syntax MySQL supports and the data validation checks it performs. Defaults to: ${mysql_mode}"`
	CharacterSetName      string                                 `placeholder:"${mysql_charset}" help:"Configures the character_set_server variable."`
	CharacterSetCollation string                                 `placeholder:"${mysql_collation}" help:"Configures the collation_server variable."`
	LongQueryTime         storage.LongQueryTime                  `placeholder:"${mysql_long_query_time}" help:"Configures the long_query_time variable. If a query takes longer than this duration, the query is logged to the slow query log file."`
	MinWordLength         *int                                   `placeholder:"${mysql_min_word_length}" help:"Configures the ft_min_word_len and innodb_ft_min_token_size variables."`
	TransactionIsolation  storage.MySQLTransactionCharacteristic `placeholder:"${mysql_transaction_isolation}" help:"Configures the transaction_isolation variable."`
	KeepDailyBackups      *int                                   `placeholder:"${mysql_backup_retention_days}" help:"Number of daily database backups to keep. Note that setting this to 0, backup will be disabled and existing dumps deleted immediately."`
}

func (cmd *mySQLCmd) Run(ctx context.Context, client *api.Client) error {
	sshkeys, err := file.ReadSSHKeys(cmd.SSHKeysFile)
	if err != nil {
		return fmt.Errorf("error when reading SSH keys file: %w", err)
	}
	cmd.SSHKeys = append(cmd.SSHKeys, sshkeys...)

	fmt.Printf("Creating new mysql. This might take some time (waiting up to %s).\n", cmd.WaitTimeout)
	mysql := cmd.newMySQL(client.Project)

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

func (cmd *mySQLCmd) newMySQL(namespace string) *storage.MySQL {
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
				MachineType:  infra.NewMachineType(cmd.MachineType),
				AllowedCIDRs: []meta.IPv4CIDR{},  // avoid missing parameter error
				SSHKeys:      []storage.SSHKey{}, // avoid missing parameter error
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

	return mySQL
}

// MySQLKongVars returns all variables which are used in the MySQL
// create command
func MySQLKongVars() kong.Vars {
	result := make(kong.Vars)
	result["mysql_machine_types"] = strings.Join(mtStringSlice(storage.MySQLMachineTypes), ", ")
	result["mysql_machine_default"] = storage.MySQLMachineTypeDefault.String()
	result["mysql_location_options"] = strings.Join(storage.MySQLLocationOptions, ", ")
	result["mysql_location_default"] = string(storage.MySQLLocationDefault)
	result["mysql_user"] = string(storage.MySQLUser)
	result["mysql_mode"] = strings.Join(storage.MySQLModeDefault, ", ")
	result["mysql_long_query_time"] = string(storage.MySQLLongQueryTimeDefault)
	result["mysql_charset"] = string(storage.MySQLCharsetDefault)
	result["mysql_collation"] = string(storage.MySQLCollationDefault)
	result["mysql_min_word_length"] = fmt.Sprintf("%d", storage.MySQLMinWordLengthDefault)
	result["mysql_transaction_isolation"] = string(storage.MySQLTransactionIsolationDefault)
	result["mysql_backup_retention_days"] = fmt.Sprintf("%d", storage.MySQLBackupRetentionDaysDefault)
	return result
}

func mtStringSlice(machineTypes []infra.MachineType) []string {
	types := make([]string, len(machineTypes))
	for i, machineType := range storage.MySQLMachineTypes {
		types[i] = machineType.String()
	}
	return types
}
