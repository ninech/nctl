package create

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/alecthomas/kong"
	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"

	"github.com/ninech/nctl/api"
)

type bucketUserCmd struct {
	resourceCmd
	Location meta.LocationName `placeholder:"${bucketuser_location_default}" help:"Where the BucketUser instance is created. Available locations are: ${bucketuser_location_options}"`
}

func (cmd *bucketUserCmd) Run(ctx context.Context, client *api.Client) error {
	fmt.Println("Creating new bucketuser.")
	bucketuser := cmd.newBucketUser(client.Project)

	c := newCreator(client, bucketuser, "bucketuser")
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !cmd.Wait {
		return nil
	}

	return c.wait(ctx, waitStage{
		objectList: &storage.BucketUserList{},
		onResult: func(event watch.Event) (bool, error) {
			if c, ok := event.Object.(*storage.BucketUser); ok {
				return isAvailable(c), nil
			}
			return false, nil
		},
	})
}

func (cmd *bucketUserCmd) newBucketUser(namespace string) *storage.BucketUser {
	name := getName(cmd.Name)

	bucketUser := &storage.BucketUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: storage.BucketUserSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      "bucketuser-" + name,
					Namespace: namespace,
				},
			},
			ForProvider: storage.BucketUserParameters{
				Location: cmd.Location,
			},
		},
	}
	return bucketUser
}

func BucketUserKongVars() kong.Vars {
	result := make(kong.Vars)
	result["bucketuser_location_default"] = string(storage.BucketUserLocationDefault)
	result["bucketuser_location_options"] = strings.Join(storage.BucketUserLocationOptions, ", ")
	return result
}
