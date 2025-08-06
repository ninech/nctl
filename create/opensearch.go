package create

import (
	"context"

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
	Location     string                        `help:"Location where the OpenSearch cluster is created." placeholder:"nine-es34"`
	MachineType  string                        `help:"MachineType specifies the type of machine to use for the OpenSearch cluster." placeholder:"nine-search-s"`
	ClusterType  storage.OpenSearchClusterType `help:"ClusterType specifies the type of OpenSearch cluster to create. Options: single, multi" placeholder:"single"`
	AllowedCidrs []meta.IPv4CIDR               `help:"AllowedCIDRs specify the allowed IP addresses, connecting to the cluster." placeholder:"203.0.113.1/32"`
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
				Location:     meta.LocationName(cmd.Location),
				MachineType:  infra.NewMachineType(cmd.MachineType),
				ClusterType:  cmd.ClusterType,
				AllowedCIDRs: cmd.AllowedCidrs,
			},
		},
	}

	return openSearch, nil
}
