package delete

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
)

type mySQLCmd struct {
	Name        string        `arg:"" help:"Name of the MySQL resource."`
	Force       bool          `default:"false" help:"Do not ask for confirmation of deletion."`
	Wait        bool          `default:"true" help:"Wait until MySQL is fully deleted."`
	WaitTimeout time.Duration `default:"300s" help:"Duration to wait for the deletion. Only relevant if wait is set."`
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
