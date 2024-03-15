package create

import (
	"context"
	"fmt"
	"time"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type redisCmd struct {
	Name            string                       `arg:"" default:"" help:"Name of the Redis instance. A random name is generated if omitted."`
	Location        string                       `default:"nine-es34" help:"Location where the Redis instance is created."`
	RedisVersion    storage.RedisVersion         `help:"Version specifies the Redis version."`
	MemorySize      string                       `help:"MemorySize configures Redis to use a specified amount of memory for the data set."`
	MaxMemoryPolicy storage.RedisMaxMemoryPolicy `help:"MaxMemoryPolicy specifies the exact behavior Redis follows when the maxmemory limit is reached."`
	AllowedCIDRs    []storage.IPv4CIDR           `help:"AllowedCIDRs specify the allowed IP addresses, connecting to the instance."`
	Wait            bool                         `default:"true" help:"Wait until Redis is created."`
	WaitTimeout     time.Duration                `default:"300s" help:"Duration to wait for Redis getting ready. Only relevant if wait is set."`
}

func (cmd *redisCmd) Run(ctx context.Context, client *api.Client) error {
	redis, err := cmd.newRedis(client.Project)
	if err != nil {
		return err
	}

	c := newCreator(client, redis, "redis")
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !cmd.Wait {
		return nil
	}

	return c.wait(ctx, waitStage{
		objectList: &storage.RedisList{},
		onResult: func(event watch.Event) (bool, error) {
			if c, ok := event.Object.(*storage.Redis); ok {
				return isAvailable(c), nil
			}
			return false, nil
		},
	},
	)
}

func (cmd *redisCmd) newRedis(namespace string) (*storage.Redis, error) {
	name := getName(cmd.Name)

	redis := &storage.Redis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: storage.RedisSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      name,
					Namespace: namespace,
				},
			},
			ForProvider: storage.RedisParameters{
				Location:        meta.LocationName(cmd.Location),
				Version:         cmd.RedisVersion,
				MaxMemoryPolicy: cmd.MaxMemoryPolicy,
				AllowedCIDRs:    cmd.AllowedCIDRs,
			},
		},
	}

	if cmd.MemorySize != "" {
		q, err := resource.ParseQuantity(cmd.MemorySize)
		if err != nil {
			return redis, fmt.Errorf("error parsing memory size %q: %w", cmd.MemorySize, err)
		}

		redis.Spec.ForProvider.MemorySize = &storage.RedisMemorySize{Quantity: q}
	}

	return redis, nil
}
