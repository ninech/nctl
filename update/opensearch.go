package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type openSearchCmd struct {
	resourceCmd
	MachineType  *string          `help:"MachineType configures OpenSearch to use a specified machine type." placeholder:"nine-search-m"`
	AllowedCidrs *[]meta.IPv4CIDR `help:"AllowedCIDRs specify the allowed IP addresses, connecting to the instance." placeholder:"203.0.113.1/32"`
}

func (cmd *openSearchCmd) Run(ctx context.Context, client *api.Client) error {
	if cmd.MachineType == nil && cmd.AllowedCidrs == nil {
		return fmt.Errorf("at least one parameter must be provided to update the OpenSearch instance")
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

func (cmd *openSearchCmd) applyUpdates(openSearch *storage.OpenSearch) error {
	if cmd.MachineType != nil {
		openSearch.Spec.ForProvider.MachineType = infra.NewMachineType(*cmd.MachineType)
	}
	if cmd.AllowedCidrs != nil {
		openSearch.Spec.ForProvider.AllowedCIDRs = *cmd.AllowedCidrs
	}

	return nil
}
