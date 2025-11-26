package update

import (
	"context"
	"fmt"

	"github.com/alecthomas/kong"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type bucketCmd struct {
	resourceCmd
	PublicRead             *bool    `help:"Publicly readable objects." default:"false"`
	PublicList             *bool    `help:"Publicly listable objects." default:"false"`
	Versioning             *bool    `help:"Enable object versioning." default:"false"`
	Permissions            []string `sep:";" placeholder:"${bucket_permissions_example}" help:"Permissions configure user access to the objects in this Bucket (repeatable: ROLE=USER[,USER...];ROLE=USER[,USER...). Available roles are: ${bucket_role_options}"`
	DeletePermissions      []string `sep:";" placeholder:"${bucket_permissions_example}" help:"Permissions which are to be deleted (repeatable: ROLE=USER[,USER...];ROLE=USER[,USER...). Available roles are: ${bucket_role_options}"`
	LifecyclePolicy        []string `placeholder:"${bucket_lifecycle_policy_example}" help:"LifecyclePolicies allows to define automatic expiry (deletion) of objects using certain rules (repeatable: pass this flag once per policy)."`
	DeleteLifecyclePolicy  []string `placeholder:"${bucket_lifecycle_policy_delete_example}" help:"LifecyclePolicies which are to be deleted (repeatable: pass this flag once per policy)."`
	ClearLifecyclePolicies bool     `help:"Remove all lifecycle policies (can be combined with other updates to clear everything before adding new items)." default:"false"`
	CORS                   []string `sep:";" placeholder:"${bucket_cors_example}" help:"CORS settings for this bucket (repeatable: ORIGINS=ORIGIN[,ORIGIN...];HEADERS=HEADER[,HEADER...];MAX_AGE)."`
	DeleteCORS             []string `sep:";" placeholder:"${bucket_cors_delete_example}" help:"CORS settings which are to be deleted (repeatable: ORIGINS=ORIGIN[,ORIGIN...];HEADERS=HEADER[,HEADER...];MAX_AGE)."`
	CustomHostnames        []string `placeholder:"${bucket_custom_hostnames_example}" help:"CustomHostnames are DNS entries under which the bucket should be accessible. This can be used to serve public objects via an own domain name. (repeatable: HOST[,HOST...])."`
	DeleteCustomHostnames  []string `placeholder:"${bucket_custom_hostnames_example}" help:"CustomHostnames which are to be deleted (repeatable: HOST[,HOST...])."`
	ClearCustomHostnames   bool     `help:"Remove all CustomHostnames (can be combined with other updates to clear everything before adding new items)."`
}

func (cmd *bucketCmd) Run(ctx context.Context, client *api.Client) error {
	b := &storage.Bucket{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}

	upd := newUpdater(client, b, storage.BucketKind, func(current resource.Managed) error {
		b, ok := current.(*storage.Bucket)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, storage.Bucket{})
		}

		return cmd.applyUpdates(b)
	})

	return upd.Update(ctx)
}

func (cmd *bucketCmd) applyUpdates(b *storage.Bucket) error {
	if cmd.PublicRead != nil {
		b.Spec.ForProvider.PublicRead = *cmd.PublicRead
	}
	if cmd.PublicList != nil {
		b.Spec.ForProvider.PublicList = *cmd.PublicList
	}
	if cmd.Versioning != nil {
		b.Spec.ForProvider.Versioning = *cmd.Versioning
	}

	var err error
	if b.Spec.ForProvider.Permissions, err = util.PatchPermissions(
		b.Spec.ForProvider.Permissions,
		cmd.Permissions,
		cmd.DeletePermissions,
	); err != nil {
		return fmt.Errorf("patching permissions error: %w", err)
	}
	if b.Spec.ForProvider.LifecyclePolicies, err = util.PatchLifecyclePolicies(
		b.Spec.ForProvider.LifecyclePolicies,
		cmd.ClearLifecyclePolicies,
		cmd.LifecyclePolicy,
		cmd.DeleteLifecyclePolicy,
	); err != nil {
		return fmt.Errorf("patching lifecycle policies error: %w", err)
	}

	if b.Spec.ForProvider.CORS, err = util.PatchCORS(b.Spec.ForProvider.CORS, cmd.CORS, cmd.DeleteCORS); err != nil {
		return fmt.Errorf("patching cors configuration error: %w", err)
	}

	if b.Spec.ForProvider.CustomHostnames, err = util.PatchCustomHostnames(
		b.Spec.ForProvider.CustomHostnames,
		cmd.ClearCustomHostnames,
		cmd.CustomHostnames,
		cmd.DeleteCustomHostnames,
	); err != nil {
		return fmt.Errorf("patching custom hostnames error: %w", err)
	}

	return nil
}

func BucketKongVars() kong.Vars {
	result := make(kong.Vars)
	result["bucket_lifecycle_policy_delete_example"] = "prefix=tmp/"
	result["bucket_cors_delete_example"] = "origins=https://example.com"
	return result
}
