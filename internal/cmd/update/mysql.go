package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mySQLCmd struct {
	ResourceCmd
	MachineType           *string          `help:"Defines the sizing for a particular MySQL instance." completion-predictor:"apifield:mysql_machine_type"`
	AllowedCidrs          *[]meta.IPv4CIDR `placeholder:"203.0.113.1/32" help:"Specifies the IP addresses allowed to connect to the instance."`
	DatabaseSSHKeysFlags  `set:"ssh_keys_purpose=allowed to connect to the database server in order to up-/download and directly restore database backups"`
	SQLMode               *[]storage.MySQLMode                    `placeholder:"\"MODE1, MODE2, ...\"" help:"Configures the sql_mode setting. Modes affect the SQL syntax MySQL supports and the data validation checks it performs. Defaults to: ${mysql_mode}"`
	CharacterSetName      *string                                 `help:"Configures the character_set_server variable." completion-predictor:"apifield:mysql_character_set_name"`
	CharacterSetCollation *string                                 `help:"Configures the collation_server variable." completion-predictor:"apifield:mysql_character_set_collation"`
	LongQueryTime         *storage.LongQueryTime                  `help:"Configures the long_query_time variable. If a query takes longer than this duration, the query is logged to the slow query log file." completion-predictor:"apifield:mysql_long_query_time"`
	MinWordLength         *int                                    `help:"Configures the ft_min_word_len and innodb_ft_min_token_size variables." completion-predictor:"apifield:mysql_min_word_length"`
	TransactionIsolation  *storage.MySQLTransactionCharacteristic `help:"Configures the transaction_isolation variable." completion-predictor:"apifield:mysql_transaction_isolation"`
	KeepDailyBackups      *int                                    `help:"Number of daily database backups to keep. Note that setting this to 0, backup will be disabled and existing dumps deleted immediately." completion-predictor:"apifield:mysql_keep_daily_backups"`
}

func (cmd *mySQLCmd) Run(ctx context.Context, client *api.Client) error {
	mysql := &storage.MySQL{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}

	upd := cmd.newUpdater(client, mysql, storage.MySQLKind, func(current resource.Managed) error {
		mysql, ok := current.(*storage.MySQL)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, storage.MySQL{})
		}

		return cmd.applyUpdates(mysql)
	})

	return upd.Update(ctx)
}

func (cmd *mySQLCmd) applyUpdates(mysql *storage.MySQL) error {
	if cmd.MachineType != nil {
		mysql.Spec.ForProvider.MachineType = infra.NewMachineType(*cmd.MachineType)
	}
	if cmd.AllowedCidrs != nil {
		mysql.Spec.ForProvider.AllowedCIDRs = *cmd.AllowedCidrs
	}
	if cmd.SSHKeysSet() {
		sshKeys, err := cmd.StorageKeys(&cmd.Writer)
		if err != nil {
			return err
		}
		mysql.Spec.ForProvider.SSHKeys = sshKeys
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

	return nil
}
