package create

import (
	"context"
	"strings"

	"github.com/alecthomas/kong"
	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type keyValueStoreCmd struct {
	resourceCmd
	Location                meta.LocationName                    `placeholder:"${keyvaluestore_location_default}" help:"Where the Key-Value Store instance is created. Available locations are: ${keyvaluestore_location_options}"`
	MemorySize              *storage.KeyValueStoreMemorySize     `placeholder:"${keyvaluestore_memorysize_default}" help:"Available amount of memory."`
	MaxMemoryPolicy         storage.KeyValueStoreMaxMemoryPolicy `placeholder:"${keyvaluestore_maxmemorypolicy_default}" help:"Behaviour when the memory limit is reached."`
	AllowedCidrs            []meta.IPv4CIDR                      `placeholder:"203.0.113.1/32" help:"IP addresses allowed to connect to the public endpoint."`
	PublicNetworkingEnabled *bool                                `help:"If public networking is \"false\", it is only possible to access the service by configuring a service connection."`
}

func (cmd *keyValueStoreCmd) Run(ctx context.Context, client *api.Client) error {
	keyValueStore, err := cmd.newKeyValueStore(client.Project)
	if err != nil {
		return err
	}

	c := newCreator(client, keyValueStore, "keyvaluestore")
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

// KeyValueStoreKongVars returns all variables which are used in the KeyValueStore
// create command.
func KeyValueStoreKongVars() kong.Vars {
	result := make(kong.Vars)
	result["keyvaluestore_memorysize_default"] = storage.KeyValueStoreMemorySizeDefault
	result["keyvaluestore_maxmemorypolicy_default"] = string(storage.KeyValueStoreMaxMemoryPolicyDefault)
	result["keyvaluestore_location_options"] = strings.Join(stringSlice(storage.KeyValueStoreLocationOptions), ", ")
	result["keyvaluestore_location_default"] = string(storage.KeyValueStoreLocationDefault)
	return result
}

func (cmd *keyValueStoreCmd) newKeyValueStore(namespace string) (*storage.KeyValueStore, error) {
	name := getName(cmd.Name)

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
				PublicNetworkingEnabled: cmd.PublicNetworkingEnabled,
				MemorySize:              cmd.MemorySize,
			},
		},
	}

	return keyValueStore, nil
}
