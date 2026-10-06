package create

import (
	"context"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type keyValueStoreCmd struct {
	ResourceCmd
	Location         meta.LocationName                    `help:"Where the Key-Value Store instance is created." completion-predictor:"apifield:keyvaluestore_location"`
	MemorySize       *storage.KeyValueStoreMemorySize     `help:"Available amount of memory." completion-predictor:"apifield:keyvaluestore_memory_size"`
	MaxMemoryPolicy  storage.KeyValueStoreMaxMemoryPolicy `help:"Behaviour when the memory limit is reached." completion-predictor:"apifield:keyvaluestore_max_memory_policy"`
	AllowedCidrs     []meta.IPv4CIDR                      `placeholder:"203.0.113.1/32" help:"IP addresses allowed to connect to the public endpoint."`
	PublicNetworking *bool                                `negatable:"" help:"Enable or disable public networking. Enabled by default."`

	// Deprecated Flags
	PublicNetworkingEnabled *bool `hidden:""`
}

func (cmd *keyValueStoreCmd) Run(ctx context.Context, client *api.Client) error {
	keyValueStore, err := cmd.newKeyValueStore(client.Project)
	if err != nil {
		return err
	}

	c := cmd.newCreator(client, keyValueStore, storage.KeyValueStoreKind)
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !cmd.Wait {
		return nil
	}

	return c.wait(ctx, waitStage{
		objectList: &storage.KeyValueStoreList{},
		onResult: func(event watch.Event) (bool, error) {
			if c, ok := event.Object.(*storage.KeyValueStore); ok {
				return isAvailable(c), nil
			}
			return false, nil
		},
	})
}

func (cmd *keyValueStoreCmd) newKeyValueStore(namespace string) (*storage.KeyValueStore, error) {
	name := getName(cmd.Name)

	publicNetworking := cmd.PublicNetworking
	if publicNetworking == nil {
		publicNetworking = cmd.PublicNetworkingEnabled
	}

	keyValueStore := &storage.KeyValueStore{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: storage.KeyValueStoreSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      "keyvaluestore-" + name,
					Namespace: namespace,
				},
			},
			ForProvider: storage.KeyValueStoreParameters{
				Location:                cmd.Location,
				MaxMemoryPolicy:         cmd.MaxMemoryPolicy,
				AllowedCIDRs:            cmd.AllowedCidrs,
				PublicNetworkingEnabled: publicNetworking,
				MemorySize:              cmd.MemorySize,
			},
		},
	}

	return keyValueStore, nil
}
