package delete

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/types"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
)

type redisCmd struct {
	Name        string        `arg:"" help:"Name of the Redis resource."`
	Force       bool          `default:"false" help:"Do not ask for confirmation of deletion."`
	Wait        bool          `default:"true" help:"Wait until Redis is fully deleted."`
	WaitTimeout time.Duration `default:"300s" help:"Duration to wait for the deletion. Only relevant if wait is set."`
}

func (cmd *redisCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	redis := &storage.Redis{}
	redisName := types.NamespacedName{Name: cmd.Name, Namespace: client.Project}
	if err := client.Get(ctx, redisName, redis); err != nil {
		return fmt.Errorf("unable to get redis %q: %w", redis.Name, err)
	}

	return newDeleter(redis, storage.RedisKind).deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force)
}
