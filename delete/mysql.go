package delete

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
)

type mySQLCmd struct {
	resourceCmd
}

func (cmd *mySQLCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	mysql := &storage.MySQL{}
	mysqlName := types.NamespacedName{Name: cmd.Name, Namespace: client.Project}
	if err := client.Get(ctx, mysqlName, mysql); err != nil {
		return fmt.Errorf("unable to get mysql %q: %w", mysql.Name, err)
	}

	return newDeleter(mysql, storage.MySQLKind).deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force)
}
