package delete

import (
	"context"
	"fmt"

	iam "github.com/ninech/apis/iam/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type apiServiceAccountCmd struct {
	resourceCmd
}

func (cmd *apiServiceAccountCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	sa := &iam.APIServiceAccount{ObjectMeta: metav1.ObjectMeta{
		Name:      cmd.Name,
		Namespace: client.Project,
	}}

	d := cmd.newDeleter(sa, iam.APIServiceAccountKind)

	if err := d.deleteResource(ctx, client, cmd.WaitTimeout, cmd.Wait, cmd.Force); err != nil {
		return fmt.Errorf("error while deleting %s: %w", iam.APIServiceAccountKind, err)
	}

	return nil
}
