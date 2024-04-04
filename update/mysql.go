package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mySQLCmd struct {
	Name                  string                                  `arg:"" default:"" help:"Name of the MySQL instance to update."`
	MachineType           *infra.MachineType                      `help:"Defines the sizing for a particular MySQL instance."`
	AllowedCidrs          *[]storage.IPv4CIDR                     `default:"" help:"Specify the allowed IP addresses, connecting to the instance."`
	SSHKeys               *[]storage.SSHKey                       `help:"Contains a list of SSH public keys, allowed to connect to the db server, in order to up-/download and directly restore database backups."`
	SQLMode               *[]storage.MySQLMode                    `help:"Configures the sql_mode setting. Modes affect the SQL syntax MySQL supports and the data validation checks it performs."`
	CharacterSetName      *string                                 `help:"Configures the character_set_server variable."`
	CharacterSetCollation *string                                 `help:"Configures the collation_server variable."`
	LongQueryTime         *storage.LongQueryTime                  `help:"Configures the long_query_time variable. If a query takes longer than this many seconds, the the query is logged to the slow query log file."`
	MinWordLength         *int                                    `help:"Configures the ft_min_word_len and innodb_ft_min_token_size variables."`
	TransactionIsolation  *storage.MySQLTransactionCharacteristic `help:"Configures the transaction_isolation variable."`
	KeepDailyBackups      *int                                    `help:"Number of daily database backups to keep. Note that setting this to 0, backup will be disabled and existing dumps deleted immediately."`
}

func (cmd *mySQLCmd) Run(ctx context.Context, client *api.Client) error {
	project := &storage.MySQL{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}

	return newUpdater(client, project, storage.MySQLKind, func(current resource.Managed) error {
		project, ok := current.(*storage.MySQL)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, storage.MySQL{})
		}

		return cmd.applyUpdates(project)
	}).Update(ctx)
}

func (cmd *mySQLCmd) applyUpdates(mysql *storage.MySQL) error {
	if cmd.MachineType != nil {
		mysql.Spec.ForProvider.MachineType = *cmd.MachineType
	}
	if cmd.AllowedCidrs != nil {
		mysql.Spec.ForProvider.AllowedCIDRs = *cmd.AllowedCidrs
	}
	if cmd.SSHKeys != nil {
		mysql.Spec.ForProvider.SSHKeys = *cmd.SSHKeys
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
