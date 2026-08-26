package create

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"

	"github.com/ninech/nctl/api"
)

type mysqlDatabaseCmd struct {
	ResourceCmd
	Location             meta.LocationName                      `help:"Where the MySQL database is created." completion-predictor:"apifield:mysqldatabase_location"`
	MysqlDatabaseVersion storage.MySQLVersion                   `help:"Version of the MySQL database." completion-predictor:"apifield:mysqldatabase_version"`
	CharacterSet         string                                 `help:"Character set for the MySQL database." completion-predictor:"apifield:mysqldatabase_character_set"`
	BackupSchedule       storage.DatabaseBackupScheduleCalendar `help:"Backup schedule for the MySQL database." completion-predictor:"apifield:mysqldatabase_backup_schedule"`
}

func (cmd *mysqlDatabaseCmd) Run(ctx context.Context, client *api.Client) error {
	mysqlDatabase := cmd.newMySQLDatabase(client.Project)

	c := cmd.newCreator(client, mysqlDatabase, storage.MySQLDatabaseKind)
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !cmd.Wait {
		return nil
	}

	if err := c.wait(ctx, waitStage{
		Writer:     cmd.Writer,
		objectList: &storage.MySQLDatabaseList{},
		onResult: func(event watch.Event) (bool, error) {
			if mdb, ok := event.Object.(*storage.MySQLDatabase); ok {
				return isAvailable(mdb), nil
			}
			return false, nil
		},
	}); err != nil {
		return err
	}

	cmd.Successf("🚀", "Your MySQLDatabase %s is now available. You can retrieve the database, username and password with:\n\n nctl get mysqldatabase %s --print-connection-string", mysqlDatabase.Name, mysqlDatabase.Name)

	return nil
}

func (cmd *mysqlDatabaseCmd) newMySQLDatabase(namespace string) *storage.MySQLDatabase {
	name := getName(cmd.Name)

	mysqlDatabase := &storage.MySQLDatabase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: storage.MySQLDatabaseSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      "mysqldatabase-" + name,
					Namespace: namespace,
				},
			},
			ForProvider: storage.MySQLDatabaseParameters{
				Location: cmd.Location,
				Version:  cmd.MysqlDatabaseVersion,
				CharacterSet: storage.MySQLCharacterSet{
					Name: cmd.CharacterSet,
				},
				BackupSchedule: cmd.BackupSchedule,
			},
		},
	}

	return mysqlDatabase
}
