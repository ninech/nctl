package create

import (
	"context"
	"strings"

	"github.com/alecthomas/kong"
	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type openSearchCmd struct {
	resourceCmd
	Location                meta.LocationName             `placeholder:"${opensearch_location_default}" help:"Where the OpenSearch cluster is created. Available locations are: ${opensearch_location_options}"`
	ClusterType             storage.OpenSearchClusterType `placeholder:"${opensearch_cluster_type_default}" help:"Type of cluster. Available types: ${opensearch_cluster_types}"`
	MachineType             string                        `placeholder:"${opensearch_machine_type_default}" help:"Defines the sizing of an OpenSearch instance. Available types: ${opensearch_machine_types}"`
	AllowedCidrs            []meta.IPv4CIDR               `placeholder:"203.0.113.1/32" help:"IP addresses allowed to connect to the public endpoint."`
	PublicNetworkingEnabled *bool                         `help:"If public networking is \"false\", it is only possible to access the service by configuring a service connection."`
	BucketUsers             []string                      `placeholder:"user1,user2" help:"BucketUsers specify the users who have read access to the OpenSearch snapshots bucket."`
}

func (cmd *openSearchCmd) Run(ctx context.Context, client *api.Client) error {
	openSearch, err := cmd.newOpenSearch(client.Project)
	if err != nil {
		return err
	}

	c := newCreator(client, openSearch, "opensearch")
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !cmd.Wait {
		return nil
	}

	return c.wait(ctx, waitStage{
		objectList: &storage.OpenSearchList{},
		onResult: func(event watch.Event) (bool, error) {
			if c, ok := event.Object.(*storage.OpenSearch); ok {
				return isAvailable(c), nil
			}
			return false, nil
		},
	})
}

// OpenSearchKongVars returns all variables which are used in the OpenSearch
// create command
func OpenSearchKongVars() kong.Vars {
	result := make(kong.Vars)
	result["opensearch_machine_types"] = strings.Join(stringerSlice(storage.OpenSearchMachineTypes), ", ")
	result["opensearch_machine_type_default"] = storage.OpenSearchMachineTypeDefault.String()
	result["opensearch_cluster_types"] = strings.Join(stringSlice(storage.OpenSearchClusterTypes), ", ")
	result["opensearch_cluster_type_default"] = string(storage.OpenSearchClusterTypeDefault)
	result["opensearch_location_options"] = strings.Join(stringSlice(storage.OpenSearchLocationOptions), ", ")
	result["opensearch_location_default"] = string(storage.OpenSearchLocationDefault)
	result["opensearch_user"] = string(storage.OpenSearchUser)
	return result
}

func (cmd *openSearchCmd) newOpenSearch(namespace string) (*storage.OpenSearch, error) {
	name := getName(cmd.Name)

	openSearch := &storage.OpenSearch{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: storage.OpenSearchSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      "opensearch-" + name,
					Namespace: namespace,
				},
			},
			ForProvider: storage.OpenSearchParameters{
				Location:                cmd.Location,
				MachineType:             infra.NewMachineType(cmd.MachineType),
				ClusterType:             cmd.ClusterType,
				AllowedCIDRs:            cmd.AllowedCidrs,
				BucketUsers:             LocalReferences(cmd.BucketUsers),
				PublicNetworkingEnabled: cmd.PublicNetworkingEnabled,
			},
		},
	}

	return openSearch, nil
}

// LocalReferences converts a slice of strings to []meta.LocalReference.
func LocalReferences(s []string) []meta.LocalReference {
	references := make([]meta.LocalReference, len(s))
	for i, user := range s {
		references[i] = meta.LocalReference{Name: user}
	}

	return references
}
