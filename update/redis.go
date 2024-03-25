package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	kresource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type redisCmd struct {
	Name            string                        `arg:"" default:"" help:"Name of the Redis instance to update."`
	MemorySize      *string                       `help:"MemorySize configures Redis to use a specified amount of memory for the data set." placeholder:"1Gi"`
	MaxMemoryPolicy *storage.RedisMaxMemoryPolicy `help:"MaxMemoryPolicy specifies the exact behavior Redis follows when the maxmemory limit is reached." placeholder:"allkeys-lru"`
	AllowedCidrs    *[]storage.IPv4CIDR           `default:"" help:"AllowedCIDRs specify the allowed IP addresses, connecting to the instance." placeholder:"0.0.0.0/0"`
}

func (cmd *redisCmd) Run(ctx context.Context, client *api.Client) error {
	redis := &storage.Redis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}

	return newUpdater(client, redis, storage.RedisKind, func(current resource.Managed) error {
		redis, ok := current.(*storage.Redis)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, storage.Redis{})
		}

		return cmd.applyUpdates(redis)
	}).Update(ctx)
}

func (cmd *redisCmd) applyUpdates(redis *storage.Redis) error {
	if cmd.MemorySize != nil {
		q, err := kresource.ParseQuantity(*cmd.MemorySize)
		if err != nil {
			return fmt.Errorf("error parsing memory size %q: %w", *cmd.MemorySize, err)
		}

		redis.Spec.ForProvider.MemorySize = &storage.RedisMemorySize{Quantity: q}
	}
	if cmd.MaxMemoryPolicy != nil {
		redis.Spec.ForProvider.MaxMemoryPolicy = *cmd.MaxMemoryPolicy
	}
	if cmd.AllowedCidrs != nil {
		redis.Spec.ForProvider.AllowedCIDRs = *cmd.AllowedCidrs
	}

	return nil
}
