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
	MachineType      *string                  `help:"Configures OpenSearch to use a specified machine type." placeholder:"nine-search-m"`
	AllowedCidrs     *[]meta.IPv4CIDR         `help:"IP addresses allowed to connect to the cluster. These restrictions do not apply for service connections." placeholder:"203.0.113.1/32"`
	BucketUsers      *[]create.LocalReference `help:"Users who have read access to the OpenSearch snapshots bucket." placeholder:"user1,user2"`
	PublicNetworking *bool                    `negatable:"" help:"Enable or disable public networking."`

	// Deprecated Flags
	PublicNetworkingEnabled *bool `hidden:"" help:"If public networking is \"false\", it is only possible to access the service by configuring a service connection."`
}

func (cmd *openSearchCmd) Run(ctx context.Context, client *api.Client) error {
	openSearch := &storage.OpenSearch{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}

	return cmd.newUpdater(client, openSearch, storage.OpenSearchKind, func(current resource.Managed) error {
		openSearch, ok := current.(*storage.OpenSearch)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, storage.OpenSearch{})
		}

		return cmd.applyUpdates(openSearch)
	}).Update(ctx)
}

func (cmd *openSearchCmd) applyUpdates(os *storage.OpenSearch) error {
	changed := false
	if cmd.MachineType != nil {
		os.Spec.ForProvider.MachineType = infra.NewMachineType(*cmd.MachineType)
		changed = true
	}
	if cmd.AllowedCidrs != nil {
		os.Spec.ForProvider.AllowedCIDRs = *cmd.AllowedCidrs
		changed = true
	}
	if cmd.BucketUsers != nil {
		bucketUsers := make([]meta.LocalReference, 0, len(*cmd.BucketUsers))
		for _, user := range *cmd.BucketUsers {
			bucketUsers = append(bucketUsers, user.LocalReference)
		}
		os.Spec.ForProvider.BucketUsers = bucketUsers
		changed = true
	}

	publicNetworking := cmd.PublicNetworking
	if publicNetworking == nil {
		publicNetworking = cmd.PublicNetworkingEnabled
	}
	if publicNetworking != nil {
		os.Spec.ForProvider.PublicNetworkingEnabled = publicNetworking
		changed = true
	}

	if !changed {
		return fmt.Errorf("no flags or arguments provided for update; please specify what you want to update (e.g. --machine-type)")
	}
	return nil
}
