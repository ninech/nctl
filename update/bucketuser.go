package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type bucketUserCmd struct {
	resourceCmd
	ResetCredentials *bool `help:"Permanently reset both the access key and the secret key of this user. This cannot be undone." placeholder:"false"`
}

func (cmd *bucketUserCmd) Run(ctx context.Context, client *api.Client) error {
	bu := &storage.BucketUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}

	upd := newUpdater(client, bu, storage.BucketUserKind, func(current resource.Managed) error {
		bu, ok := current.(*storage.BucketUser)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, storage.BucketUser{})
		}

		bu.Spec.ForProvider.ResetCredentials = cmd.ResetCredentials
		return nil
	})

	return upd.Update(ctx)
}
