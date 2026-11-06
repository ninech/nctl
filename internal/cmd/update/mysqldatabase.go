package update

import (
	"context"
	"fmt"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mysqlDatabaseCmd struct {
	ResourceCmd
	CredentialsResetFlags
	BackupSchedule  *storage.DatabaseBackupScheduleCalendar `help:"Backup schedule for the MySQL database." completion-predictor:"apifield:mysqldatabase_backup_schedule"`
	BackupDailyTime *string                                 `help:"Time of day in the format HH:MM (Europe/Zurich) at which daily backups of the MySQL database are created."`
	BackupTTL       *time.Duration                          `help:"Duration after which scheduled backups of the MySQL database can be deleted. Must be between 24h and 720h. Set to 0 to reset to the default." completion-predictor:"apifield:mysqldatabase_backup_ttl"`
}

func (cmd *mysqlDatabaseCmd) Run(ctx context.Context, client *api.Client) error {
	mysqlDatabase := &storage.MySQLDatabase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}

	upd := cmd.newUpdater(client, mysqlDatabase, storage.MySQLDatabaseKind, func(current resource.Managed) error {
		mysqlDatabase, ok := current.(*storage.MySQLDatabase)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, storage.MySQLDatabase{})
		}

		cmd.applyUpdates(mysqlDatabase)
		return nil
	}).withCredentialsReset(&cmd.CredentialsResetFlags)

	return upd.Update(ctx)
}

func (cmd *mysqlDatabaseCmd) applyUpdates(db *storage.MySQLDatabase) {
	if cmd.BackupSchedule != nil {
		db.Spec.ForProvider.BackupSchedule = *cmd.BackupSchedule
	}
	if cmd.BackupDailyTime != nil {
		db.Spec.ForProvider.BackupDailyTime = *cmd.BackupDailyTime
	}
	if cmd.BackupTTL != nil {
		if *cmd.BackupTTL == 0 {
			db.Spec.ForProvider.BackupTTL = nil
		} else {
			db.Spec.ForProvider.BackupTTL = &metav1.Duration{Duration: *cmd.BackupTTL}
		}
	}
}
