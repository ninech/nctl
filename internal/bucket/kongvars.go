package bucket

import (
	"fmt"
	"strings"

	"github.com/alecthomas/kong"
	storage "github.com/ninech/apis/storage/v1alpha1"
)

// roles are the roles a bucket permission can grant.
var roles = []storage.BucketRole{storage.BucketRoleReader, storage.BucketRoleWriter}

// KongVars returns the variables which the flags of the bucket create and update commands interpolate:
// the available roles and the examples of the flag syntaxes they share.
func KongVars() kong.Vars {
	roleNames := make([]string, 0, len(roles))
	for _, role := range roles {
		roleNames = append(roleNames, string(role))
	}

	result := make(kong.Vars)
	result["bucket_role_options"] = strings.Join(roleNames, ", ")
	result["bucket_permissions_example"] = fmt.Sprintf("%s=frontend,analytics;%s=ingest", storage.BucketRoleReader, storage.BucketRoleWriter)
	result["bucket_lifecycle_policy_example"] = "prefix=p/;expire-after-days=7;is-live=true"
	result["bucket_cors_example"] = "origins=https://a.com,https://b.com;allowed-headers=Content-Type,Content-MD5;response-headers=ETag,Last-Modified;max-age=3600"
	result["bucket_custom_hostnames_example"] = "my-bucket.example.com,your-bucket.example.com"
	return result
}
