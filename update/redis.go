package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type redisCmd struct {
	Name            string                        `arg:"" default:"" help:"Name of the Redis instance to update."`
	MemorySize      *storage.RedisMemorySize      `default:"" help:"MemorySize configures Redis to use a specified amount of memory for the data set."`
	MaxMemoryPolicy *storage.RedisMaxMemoryPolicy `default:"" help:"MaxMemoryPolicy specifies the exact behavior Redis follows when the maxmemory limit is reached."`
	AllowedCIDRs    *[]storage.IPv4CIDR           `default:"" help:"AllowedCIDRs specify the allowed IP addresses, connecting to the instance."`
}

func (cmd *redisCmd) Run(ctx context.Context, client *api.Client) error {
	project := &storage.Redis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}

	return newUpdater(client, project, storage.RedisKind, func(current resource.Managed) error {
		project, ok := current.(*storage.Redis)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, storage.Redis{})
		}

		cmd.applyUpdates(project)

		return nil
	}).Update(ctx)
}

func (cmd *redisCmd) applyUpdates(redis *storage.Redis) {
	if cmd.MemorySize != nil {
		redis.Spec.ForProvider.MemorySize = cmd.MemorySize
	}
	if cmd.MaxMemoryPolicy != nil {
		redis.Spec.ForProvider.MaxMemoryPolicy = *cmd.MaxMemoryPolicy
	}
	if cmd.AllowedCIDRs != nil {
		redis.Spec.ForProvider.AllowedCIDRs = *cmd.AllowedCIDRs
	}
}
