package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/create"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type openSearchCmd struct {
	resourceCmd
	MachineType             *string          `help:"Configures OpenSearch to use a specified machine type." placeholder:"nine-search-m"`
	AllowedCidrs            *[]meta.IPv4CIDR `help:"IP addresses allowed to connect to the cluster. These restrictions do not apply for service connections." placeholder:"203.0.113.1/32"`
	BucketUsers             *[]string        `help:"Users who have read access to the OpenSearch snapshots bucket." placeholder:"user1,user2"`
	PublicNetworkingEnabled *bool            `help:"If public networking is \"false\", it is only possible to access the service by configuring a service connection." placeholder:"true"`
}

func (cmd *openSearchCmd) Run(ctx context.Context, client *api.Client) error {
	if cmd.MachineType == nil && cmd.AllowedCidrs == nil && cmd.BucketUsers == nil {
		return fmt.Errorf("at least one parameter must be provided to update the OpenSearch cluster")
	}

	openSearch := &storage.OpenSearch{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}

	return newUpdater(client, openSearch, storage.OpenSearchKind, func(current resource.Managed) error {
		openSearch, ok := current.(*storage.OpenSearch)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, storage.OpenSearch{})
		}

		return cmd.applyUpdates(openSearch)
	}).Update(ctx)
}

func (cmd *openSearchCmd) applyUpdates(os *storage.OpenSearch) error {
	if cmd.MachineType != nil {
		os.Spec.ForProvider.MachineType = infra.NewMachineType(*cmd.MachineType)
	}
	if cmd.AllowedCidrs != nil {
		os.Spec.ForProvider.AllowedCIDRs = *cmd.AllowedCidrs
	}
	if cmd.BucketUsers != nil {
		os.Spec.ForProvider.BucketUsers = create.LocalReferences(*cmd.BucketUsers)
	}
	if cmd.PublicNetworkingEnabled != nil {
		os.Spec.ForProvider.PublicNetworkingEnabled = cmd.PublicNetworkingEnabled
	}

	return nil
}
