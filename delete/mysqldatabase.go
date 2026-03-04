package delete

import (
	"context"
	"fmt"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type mysqlDatabaseCmd struct {
	resourceCmd
}

func (cmd *mysqlDatabaseCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	mysqlDatabase := &storage.MySQLDatabase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}

	d := cmd.newDeleter(
		mysqlDatabase,
		storage.MySQLDatabaseKind,
		prompt(mysqlDatabaseDeletePrompt()),
	)

	if err := d.deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force); err != nil {
		return fmt.Errorf("error while deleting %s: %w", storage.MySQLDatabaseKind, err)
	}

	return nil
}

func mysqlDatabaseDeletePrompt() promptFunc {
	return func(kind, name string) string {
		return fmt.Sprintf("Deleting %s %q will also destroy its default backup bucket and all backups in it."+
			"\n Make sure to create a copy of them first if you still need them."+
			"\n\n !!! This can not be recovered !!! \n\n"+
			"Do you really want to continue?",
			kind,
			name,
		)
	}
}
