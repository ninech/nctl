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

type postgresDatabaseCmd struct {
	ResourceCmd
	Location                meta.LocationName                      `help:"Where the PostgreSQL database is created." completion-predictor:"apifield:postgresdatabase_location"`
	PostgresDatabaseVersion storage.PostgresVersion                `help:"Release version with which the PostgreSQL database is created." completion-predictor:"apifield:postgresdatabase_version"`
	BackupSchedule          storage.DatabaseBackupScheduleCalendar `help:"Backup schedule for the PostgreSQL database." completion-predictor:"apifield:postgresdatabase_backup_schedule"`
	Collation               storage.PostgresDatabaseCollation      `help:"Collation for the PostgreSQL database." completion-predictor:"apifield:postgresdatabase_collation"`
}

func (cmd *postgresDatabaseCmd) Run(ctx context.Context, client *api.Client) error {
	postgresDatabase := cmd.newPostgresDatabase(client.Project)

	c := cmd.newCreator(client, postgresDatabase, storage.PostgresDatabaseKind)
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
		objectList: &storage.PostgresDatabaseList{},
		onResult: func(event watch.Event) (bool, error) {
			if pdb, ok := event.Object.(*storage.PostgresDatabase); ok {
				return isAvailable(pdb), nil
			}
			return false, nil
		},
	}); err != nil {
		return err
	}

	cmd.Successf("🚀", "Your PostgresDatabase %s is now available. You can retrieve the database, username and password with:\n\n nctl get postgresdatabase %s --print-connection-string",
		postgresDatabase.Name,
		postgresDatabase.Name,
	)

	return nil
}

func (cmd *postgresDatabaseCmd) newPostgresDatabase(namespace string) *storage.PostgresDatabase {
	name := getName(cmd.Name)

	postgresDatabase := &storage.PostgresDatabase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: storage.PostgresDatabaseSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      "postgresdatabase-" + name,
					Namespace: namespace,
				},
			},
			ForProvider: storage.PostgresDatabaseParameters{
				Location:       cmd.Location,
				Version:        cmd.PostgresDatabaseVersion,
				BackupSchedule: cmd.BackupSchedule,
				Collation:      cmd.Collation,
			},
		},
	}

	return postgresDatabase
}
