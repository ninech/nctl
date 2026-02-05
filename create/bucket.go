package create

import (
	"context"
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
)

type bucketCmd struct {
	resourceCmd
	Location   meta.LocationName `placeholder:"${bucket_location_default}" help:"Where the Bucket instance is created. Available locations are: ${bucket_location_options}" required:""`
	PublicRead bool              `help:"Publicly readable objects." default:"false"`
	PublicList bool              `help:"Publicly listable objects." default:"false"`
	Versioning bool              `help:"Enable object versioning." default:"false"`
	// The `sep:";"` option disables Kong's default comma-splitting and instead
	// allows multiple ROLE=USER[,USER...] segments to be provided in a single flag,
	// separated by semicolons. Example:
	//   --permissions=reader=alice,bob
	//   --permissions=writer=john;reader=carol
	Permissions     []string `sep:";" placeholder:"${bucket_permissions_example}" help:"Permissions configure user access to the objects in this Bucket (repeatable: ROLE=USER[,USER...];ROLE=USER[,USER...). Available roles are: ${bucket_role_options}"`
	LifecyclePolicy []string `placeholder:"${bucket_lifecycle_policy_example}" help:"LifecyclePolicies allows to define automatic expiry (deletion) of objects using certain rules (repeatable: pass this flag once per policy)."`
	CORS            []string `sep:";" placeholder:"${bucket_cors_example}" help:"CORS settings for this bucket (repeatable: ORIGINS=ORIGIN[,ORIGIN...];HEADERS=HEADER[,HEADER...];MAX_AGE)."`
	CustomHostnames []string `placeholder:"${bucket_custom_hostnames_example}" help:"CustomHostnames are DNS entries under which the bucket should be accessible. This can be used to serve public objects via an own domain name. (repeatable: HOST[,HOST...])."`
}

func (cmd *bucketCmd) Run(ctx context.Context, client *api.Client) error {
	bucket, err := cmd.newBucket(client.Project)
	if err != nil {
		return err
	}

	c := cmd.newCreator(client, bucket, storage.BucketKind)
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}
	if !cmd.Wait {
		return nil
	}

	return c.wait(ctx, waitStage{
		objectList: &storage.BucketList{},
		onResult: func(event watch.Event) (bool, error) {
			if b, ok := event.Object.(*storage.Bucket); ok {
				return isAvailable(b), nil
			}
			return false, nil
		},
	})
}

func baseBucketParameters() storage.BucketParameters {
	return storage.BucketParameters{}
}

func (cmd *bucketCmd) newBucket(project string) (*storage.Bucket, error) {
	b := &storage.Bucket{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getName(cmd.Name),
			Namespace: project,
		},
		Spec: storage.BucketSpec{
			ForProvider: baseBucketParameters(),
		},
	}
	b.Spec.ForProvider = storage.BucketParameters{
		Location:   meta.LocationName(cmd.Location),
		PublicRead: cmd.PublicRead,
		PublicList: cmd.PublicList,
		Versioning: cmd.Versioning,
	}

	var err error
	if b.Spec.ForProvider.Permissions, err = util.PatchPermissions(b.Spec.ForProvider.Permissions, cmd.Permissions, nil); err != nil {
		return nil, fmt.Errorf("patching permissions error: %w", err)
	}

	if b.Spec.ForProvider.LifecyclePolicies, err = util.PatchLifecyclePolicies(
		b.Spec.ForProvider.LifecyclePolicies,
		false,
		cmd.LifecyclePolicy,
		nil,
	); err != nil {
		return nil, fmt.Errorf("patching lifecycle policies error: %w", err)
	}

	if b.Spec.ForProvider.CORS, err = util.PatchCORS(b.Spec.ForProvider.CORS, cmd.CORS, nil); err != nil {
		return nil, fmt.Errorf("patching cors error: %w", err)
	}

	if b.Spec.ForProvider.CustomHostnames, err = util.PatchCustomHostnames(
		b.Spec.ForProvider.CustomHostnames,
		false,
		cmd.CustomHostnames,
		nil,
	); err != nil {
		return nil, fmt.Errorf("patching custom hostnames error: %w", err)
	}

	return b, nil
}

func BucketKongVars() kong.Vars {
	result := make(kong.Vars)
	result["bucket_location_default"] = string(storage.BucketUserLocationDefault)
	result["bucket_location_options"] = strings.Join(storage.BucketLocationOptions, ", ")

	roles := []storage.BucketRole{storage.BucketRoleReader, storage.BucketRoleWriter}
	result["bucket_role_options"] = strings.Join(util.ToStrings(roles), ", ")
	result["bucket_permissions_example"] = fmt.Sprintf("%s=frontend,analytics;%s=ingest", storage.BucketRoleReader, storage.BucketRoleWriter)
	result["bucket_lifecycle_policy_example"] = "prefix=p/;expire-after-days=7;is-live=true"
	result["bucket_cors_example"] = "origins=https://a.com,https://b.com;response-headers=X-My-Header,ETag;max-age=3600"
	result["bucket_custom_hostnames_example"] = "my-bucket.example.com,your-bucket.example.com"
	return result
}
