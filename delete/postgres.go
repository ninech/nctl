package delete

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
)

type postgresCmd struct {
	Name        string        `arg:"" help:"Name of the PostgreSQL resource."`
	Force       bool          `default:"false" help:"Do not ask for confirmation of deletion."`
	Wait        bool          `default:"true" help:"Wait until PostgreSQL is fully deleted."`
	WaitTimeout time.Duration `default:"300s" help:"Duration to wait for the deletion. Only relevant if wait is set."`
}

func (cmd *postgresCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	postgres := &storage.Postgres{}
	postgresName := types.NamespacedName{Name: cmd.Name, Namespace: client.Project}
	if err := client.Get(ctx, postgresName, postgres); err != nil {
		return fmt.Errorf("unable to get postgres %q: %w", postgres.Name, err)
	}

	return newDeleter(postgres, storage.PostgresKind).deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force)
}
