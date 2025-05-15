package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/file"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mySQLCmd struct {
	resourceCmd
	MachineType           *string                                 `placeholder:"${mysql_machine_default}" help:"Defines the sizing for a particular MySQL instance. Available types: ${mysql_machine_types}"`
	AllowedCidrs          *[]meta.IPv4CIDR                        `placeholder:"203.0.113.1/32" help:"Specifies the IP addresses allowed to connect to the instance." `
	SSHKeys               []storage.SSHKey                        `help:"Contains a list of SSH public keys, allowed to connect to the db server, in order to up-/download and directly restore database backups."`
	SSHKeysFile           string                                  `help:"Path to a file containing a list of SSH public keys (see above), separated by newlines."`
	SQLMode               *[]storage.MySQLMode                    `placeholder:"\"MODE1, MODE2, ...\"" help:"Configures the sql_mode setting. Modes affect the SQL syntax MySQL supports and the data validation checks it performs. Defaults to: ${mysql_mode}"`
	CharacterSetName      *string                                 `placeholder:"${mysql_charset}" help:"Configures the character_set_server variable."`
	CharacterSetCollation *string                                 `placeholder:"${mysql_collation}" help:"Configures the collation_server variable."`
	LongQueryTime         *storage.LongQueryTime                  `placeholder:"${mysql_long_query_time}" help:"Configures the long_query_time variable. If a query takes longer than this duration, the query is logged to the slow query log file."`
	MinWordLength         *int                                    `placeholder:"${mysql_min_word_length}" help:"Configures the ft_min_word_len and innodb_ft_min_token_size variables."`
	TransactionIsolation  *storage.MySQLTransactionCharacteristic `placeholder:"${mysql_transaction_isolation}" help:"Configures the transaction_isolation variable."`
	KeepDailyBackups      *int                                    `placeholder:"${mysql_backup_retention_days}" help:"Number of daily database backups to keep. Note that setting this to 0, backup will be disabled and existing dumps deleted immediately."`
}

func (cmd *mySQLCmd) Run(ctx context.Context, client *api.Client) error {
	mysql := &storage.MySQL{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}

	upd := newUpdater(client, mysql, storage.MySQLKind, func(current resource.Managed) error {
		mysql, ok := current.(*storage.MySQL)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, storage.MySQL{})
		}

		sshkeys, err := file.ReadSSHKeys(cmd.SSHKeysFile)
		if err != nil {
			return fmt.Errorf("error when reading SSH keys file: %w", err)
		}
		cmd.SSHKeys = append(cmd.SSHKeys, sshkeys...)

		cmd.applyUpdates(mysql)
		return nil
	})

	return upd.Update(ctx)
}

func (cmd *mySQLCmd) applyUpdates(mysql *storage.MySQL) {
	if cmd.MachineType != nil {
		mysql.Spec.ForProvider.MachineType = infra.NewMachineType(*cmd.MachineType)
	}
	if cmd.AllowedCidrs != nil {
		mysql.Spec.ForProvider.AllowedCIDRs = *cmd.AllowedCidrs
	}
	if cmd.SSHKeys != nil {
		mysql.Spec.ForProvider.SSHKeys = cmd.SSHKeys
	}
	if cmd.SQLMode != nil {
		mysql.Spec.ForProvider.SQLMode = cmd.SQLMode
	}
	if cmd.CharacterSetName != nil {
		mysql.Spec.ForProvider.CharacterSet.Name = *cmd.CharacterSetName
	}
	if cmd.CharacterSetCollation != nil {
		mysql.Spec.ForProvider.CharacterSet.Collation = *cmd.CharacterSetCollation
	}
	if cmd.LongQueryTime != nil {
		mysql.Spec.ForProvider.LongQueryTime = *cmd.LongQueryTime
	}
	if cmd.MinWordLength != nil {
		mysql.Spec.ForProvider.MinWordLength = cmd.MinWordLength
	}
	if cmd.TransactionIsolation != nil {
		mysql.Spec.ForProvider.TransactionIsolation = *cmd.TransactionIsolation
	}
	if cmd.KeepDailyBackups != nil {
		mysql.Spec.ForProvider.KeepDailyBackups = cmd.KeepDailyBackups
	}
}
