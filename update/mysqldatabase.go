package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mysqlDatabaseCmd struct {
	resourceCmd
	BackupSchedule *storage.DatabaseBackupScheduleCalendar `help:"Backup schedule for the MySQL database. Available schedules: ${mysqldatabase_backupschedule_options}"`
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
	})

	return upd.Update(ctx)
}

func (cmd *mysqlDatabaseCmd) applyUpdates(db *storage.MySQLDatabase) {
	if cmd.BackupSchedule != nil {
		db.Spec.ForProvider.BackupSchedule = *cmd.BackupSchedule
	}
}
