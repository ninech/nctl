package update

import (
	"maps"
	"testing"
	"time"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/create"

	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBucket(t *testing.T) {
	baseBucketParameters := storage.BucketParameters{}
	for name, tc := range map[string]struct {
		flags           []string
		origForProvider *storage.BucketParameters
		wantForProvider *storage.BucketParameters
		afterAssert     func(t *testing.T, b *storage.Bucket)
		wantErr         bool
	}{
		"add-permissions-via-multiple-flags": {
			flags: []string{
				"--permissions=reader=frontend,analytics",
				"--permissions=writer=ingest",
				"--permissions=reader=john",
			},
			origForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.Permissions = []*storage.BucketPermission{
					{
						Role: storage.BucketRoleReader,
						BucketUserRefs: []*meta.LocalReference{
							{Name: "user1"},
						},
					},
				}
				return &p
			}(),
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.Permissions = []*storage.BucketPermission{
					{
						Role: storage.BucketRoleReader,
						BucketUserRefs: []*meta.LocalReference{
							{Name: "analytics"},
							{Name: "frontend"},
							{Name: "john"},
							{Name: "user1"},
						},
					},
					{
						Role: storage.BucketRoleWriter,
						BucketUserRefs: []*meta.LocalReference{
							{Name: "ingest"},
						},
					},
				}
				return &p
			}(),
		},
		"add-multiple-permissions-in-one-flag": {
			flags: []string{
				"--permissions=reader=frontend,analytics;writer=ingest;reader=john",
			},
			origForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.Permissions = []*storage.BucketPermission{
					{
						Role: storage.BucketRoleReader,
						BucketUserRefs: []*meta.LocalReference{
							{Name: "user1"},
						},
					},
				}
				return &p
			}(),
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.Permissions = []*storage.BucketPermission{
					{
						Role: storage.BucketRoleReader,
						BucketUserRefs: []*meta.LocalReference{
							{Name: "analytics"},
							{Name: "frontend"},
							{Name: "john"},
							{Name: "user1"},
						},
					},
					{
						Role: storage.BucketRoleWriter,
						BucketUserRefs: []*meta.LocalReference{
							{Name: "ingest"},
						},
					},
				}
				return &p
			}(),
		},
		"add-and-delete-permissions-mixed-usage": {
			flags: []string{
				"--permissions=reader=frontend,analytics;writer=ingest;reader=john",
				"--delete-permissions=reader=user1",
			},
			origForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.Permissions = []*storage.BucketPermission{
					{
						Role: storage.BucketRoleReader,
						BucketUserRefs: []*meta.LocalReference{
							{Name: "user1"},
						},
					},
				}
				return &p
			}(),
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.Permissions = []*storage.BucketPermission{
					{
						Role: storage.BucketRoleReader,
						BucketUserRefs: []*meta.LocalReference{
							{Name: "analytics"},
							{Name: "frontend"},
							{Name: "john"},
						},
					},
					{
						Role: storage.BucketRoleWriter,
						BucketUserRefs: []*meta.LocalReference{
							{Name: "ingest"},
						},
					},
				}
				return &p
			}(),
		},
		// if you add and remove the same user in a single update (e.g.
		// --permissions=reader=user1 and --delete-permissions=reader=user1),
		// the operations cancel each other out. Order of flags doesn't matter.
		// The role stays present, but with an empty user list (e.g. reader=).
		"permissions-no-op": {
			flags: []string{
				"--delete-permissions=reader=user1",
				"--permissions=reader=user1",
			},
			origForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				return &p
			}(),
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.Permissions = []*storage.BucketPermission{
					{
						Role:           storage.BucketRoleReader,
						BucketUserRefs: nil,
					},
				}
				return &p
			}(),
		},
		"remove-specific-users-or-roles-from-permissions": {
			flags: []string{
				"--delete-permissions=reader=john",
				"--delete-permissions=writer=",
				// non existent users are simply ignored
				"--delete-permissions=reader=non-existent-user",
				"--delete-permissions=reader=another-non-existent-user",
			},
			origForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.Permissions = []*storage.BucketPermission{
					{
						Role: storage.BucketRoleReader,
						BucketUserRefs: []*meta.LocalReference{
							{Name: "john"},
						},
					},
					{
						Role: storage.BucketRoleWriter,
						BucketUserRefs: []*meta.LocalReference{
							{Name: "ingest"},
						},
					},
				}
				return &p
			}(),
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.Permissions = []*storage.BucketPermission{
					{
						Role:           storage.BucketRoleReader,
						BucketUserRefs: nil,
					},
				}
				return &p
			}(),
		},

		"add-lifecycle-policy": {
			flags: []string{
				"--lifecycle-policy=prefix=logs/;expire-after-days=7;is-live=true",
			},
			origForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.LifecyclePolicies = []*storage.BucketLifecyclePolicy{
					{
						Prefix:          "tmp/",
						ExpireAfterDays: int32(14),
						IsLive:          true,
					},
				}
				return &p
			}(),
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.LifecyclePolicies = []*storage.BucketLifecyclePolicy{
					{
						Prefix:          "logs/",
						ExpireAfterDays: int32(7),
						IsLive:          true,
					},
					{
						Prefix:          "tmp/",
						ExpireAfterDays: int32(14),
						IsLive:          true,
					},
				}
				return &p
			}(),
		},
		"add-two-remove-one-lifecycle-policy-in-one-go": {
			flags: []string{
				"--lifecycle-policy=prefix=logs/;expire-after-days=7;is-live=true",
				"--lifecycle-policy=prefix=archive/;expire-after-days=365;is-live=false",
				"--delete-lifecycle-policy=prefix=tmp/",
			},
			origForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.LifecyclePolicies = []*storage.BucketLifecyclePolicy{
					{
						Prefix:          "tmp/",
						ExpireAfterDays: int32(14),
						IsLive:          true,
					},
				}
				return &p
			}(),
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.LifecyclePolicies = []*storage.BucketLifecyclePolicy{
					{
						Prefix:          "archive/",
						ExpireAfterDays: int32(365),
						IsLive:          false,
					},
					{
						Prefix:          "logs/",
						ExpireAfterDays: int32(7),
						IsLive:          true,
					},
				}
				return &p
			}(),
		},
		"remove-a-specific-lifecycle-policy": {
			flags: []string{
				"--delete-lifecycle-policy=prefix=logs/;expire-after-days=7;is-live=true",
				"--delete-lifecycle-policy=prefix=archive/",
				"--delete-lifecycle-policy=prefix=tmp/",
			},
			origForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.LifecyclePolicies = []*storage.BucketLifecyclePolicy{
					{
						Prefix:          "archive/",
						ExpireAfterDays: int32(365),
						IsLive:          false,
					},
					{
						Prefix:          "logs/",
						ExpireAfterDays: int32(7),
						IsLive:          true,
					},
					{
						Prefix:          "tmp/",
						ExpireAfterDays: int32(14),
						IsLive:          true,
					},
				}
				return &p
			}(),
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.LifecyclePolicies = nil
				return &p
			}(),
		},
		"clear-all-policies-then-add-fresh-ones-single-run": {
			flags: []string{
				"--clear-lifecycle-policies",
				"--lifecycle-policy=prefix=logs/;expire-after-days=7;is-live=true",
				"--lifecycle-policy=prefix=archive/;expire-after-days=365;is-live=false",
			},
			origForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.LifecyclePolicies = []*storage.BucketLifecyclePolicy{
					{
						Prefix:          "tmp/",
						ExpireAfterDays: int32(14),
						IsLive:          true,
					},
					{
						Prefix:          "tmp2/",
						ExpireAfterDays: int32(14),
						IsLive:          true,
					},
				}
				return &p
			}(),
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.LifecyclePolicies = []*storage.BucketLifecyclePolicy{
					{
						Prefix:          "archive/",
						ExpireAfterDays: int32(365),
						IsLive:          false,
					},
					{
						Prefix:          "logs/",
						ExpireAfterDays: int32(7),
						IsLive:          true,
					},
				}
				return &p
			}(),
		},
		"lifecycle-policy-no-op": {
			flags: []string{
				"--lifecycle-policy=prefix=tmp/;expire-after-days=7;is-live=true",
				"--delete-lifecycle-policy=prefix=tmp/",
			},
			origForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				return &p
			}(),
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.LifecyclePolicies = nil
				return &p
			}(),
		},
		"add-and-merge-cors-single-flag": {
			flags: []string{
				"--cors=origins=https://example.com,https://app.example.com;response-headers=X-My-Header,ETag;max-age=1800",
			},
			origForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.CORS = &storage.CORSConfig{
					Origins: []string{
						"https://init.example.com",
					},
					ResponseHeaders: []string{
						"X-My-Init-Header",
					},
					MaxAge: 3600,
				}
				return &p
			}(),
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.CORS = &storage.CORSConfig{
					Origins: []string{
						"https://app.example.com",
						"https://example.com",
						"https://init.example.com",
					},
					ResponseHeaders: []string{
						"ETag",
						"X-My-Header",
						"X-My-Init-Header",
					},
					MaxAge: 1800,
				}
				return &p
			}(),
		},
		// Same outcome (as the test case above) using duplicated keys (here two
		// `origins` get merged)
		"add-and-merge-cors-single-flag-duplicate-keys": {
			flags: []string{
				"--cors=origins=https://example.com;response-headers=X-My-Header;max-age=1800;origins=https://app.example.com;response-headers=ETag",
			},
			origForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.CORS = &storage.CORSConfig{
					Origins: []string{
						"https://init.example.com",
					},
					ResponseHeaders: []string{
						"X-My-Init-Header",
					},
					MaxAge: 3600,
				}
				return &p
			}(),
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.CORS = &storage.CORSConfig{
					Origins: []string{
						"https://app.example.com",
						"https://example.com",
						"https://init.example.com",
					},
					ResponseHeaders: []string{
						"ETag",
						"X-My-Header",
						"X-My-Init-Header",
					},
					MaxAge: 1800,
				}
				return &p
			}(),
		},
		// Multiple flags. Same output as above, and still merged deterministically.
		"multiple-cors-flags": {
			flags: []string{
				"--cors=origins=https://example.com;response-headers=ETag",
				"--cors=origins=https://app.example.com;response-headers=X-My-Header",
				"--cors=max-age=1800",
			},
			origForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.CORS = &storage.CORSConfig{
					Origins: []string{
						"https://init.example.com",
					},
					ResponseHeaders: []string{
						"X-My-Init-Header",
					},
					MaxAge: 3600,
				}
				return &p
			}(),
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.CORS = &storage.CORSConfig{
					Origins: []string{
						"https://app.example.com",
						"https://example.com",
						"https://init.example.com",
					},
					ResponseHeaders: []string{
						"ETag",
						"X-My-Header",
						"X-My-Init-Header",
					},
					MaxAge: 1800,
				}
				return &p
			}(),
		},
		"delete-specific-cors-entries": {
			flags: []string{
				"--delete-cors=origins=https://app.example.com;response-headers=ETag",
				"--delete-cors=origins=https://init.example.com",
				"--delete-cors=response-headers=X-My-Init-Header",
				// non-existent should just be ignored:
				"--delete-cors=response-headers=non-existent-header;origins=https://non-existent.example.com",
			},
			origForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.CORS = &storage.CORSConfig{
					Origins: []string{
						"https://app.example.com",
						"https://example.com",
						"https://init.example.com",
					},
					ResponseHeaders: []string{
						"ETag",
						"X-My-Header",
						"X-My-Init-Header",
					},
					MaxAge: 1800,
				}
				return &p
			}(),
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.CORS = &storage.CORSConfig{
					Origins: []string{
						"https://example.com",
					},
					ResponseHeaders: []string{
						"X-My-Header",
					},
					MaxAge: 1800,
				}
				return &p
			}(),
		},
		"delete-cors-clears-when-all-origins-removed": {
			flags: []string{
				// Remove every origin that exists interpret as "clear CORS".
				"--delete-cors=origins=https://a.com,https://b.com",
			},
			origForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.CORS = &storage.CORSConfig{
					Origins: []string{
						"https://a.com",
						"https://b.com",
					},
					ResponseHeaders: []string{"ETag"},
					MaxAge:          3600,
				}
				return &p
			}(),
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.CORS = nil
				return &p
			}(),
		},
		"delete-cors-with-max-age-rejected": {
			flags: []string{
				// Not allowed: max-age cannot be used with --delete-cors
				"--delete-cors=max-age=1800",
			},
			origForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.CORS = &storage.CORSConfig{
					Origins:         []string{"https://example.com"},
					ResponseHeaders: []string{"ETag"},
					MaxAge:          3600,
				}
				return &p
			}(),
			wantErr: true,
		},
		"add-hostnames": {
			flags: []string{
				"--custom-hostnames=cdn.example.com,assets.example.com",
			},
			origForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.CustomHostnames = []string{
					"my-bucket.example.com",
				}
				return &p
			}(),
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.CustomHostnames = []string{
					"assets.example.com",
					"cdn.example.com",
					"my-bucket.example.com",
				}
				return &p
			}(),
		},
		"remove-hostnames": {
			flags: []string{
				"--delete-custom-hostnames=app.example.com",
				"--delete-custom-hostnames=assets.example.com",
			},
			origForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.CustomHostnames = []string{
					"assets.example.com",
					"app.example.com",
					"my-bucket.example.com",
				}
				return &p
			}(),
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.CustomHostnames = []string{
					"my-bucket.example.com",
				}
				return &p
			}(),
		},
		"clear-all-custom-hostnames-and-set-new": {
			flags: []string{
				"--clear-custom-hostnames",
				"--custom-hostnames=app.example.com",
			},
			origForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.CustomHostnames = []string{
					"my-bucket.example.com",
				}
				return &p
			}(),
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.CustomHostnames = []string{
					"app.example.com",
				}
				return &p
			}(),
		},
		"update-all": {
			flags: []string{
				"--public-read=false",
				"--public-list=true",
				"--versioning=false",
				"--permissions=reader=user2",
				"--delete-permissions=reader=user1",

				"--lifecycle-policy=prefix=tmp/;expire-after-days=1",
				"--delete-lifecycle-policy=prefix=logs/",

				"--cors=max-age=1800",
				"--delete-cors=response-headers=X-My-Header-2",
				"--delete-cors=origins=https://example2.com;",

				"--clear-custom-hostnames",
			},
			origForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.PublicRead = true
				p.PublicList = false
				p.Versioning = true
				p.Permissions = []*storage.BucketPermission{
					{
						Role: storage.BucketRoleReader,
						BucketUserRefs: []*meta.LocalReference{
							{Name: "user1"},
						},
					},
				}
				p.LifecyclePolicies = []*storage.BucketLifecyclePolicy{
					{
						Prefix:          "logs/",
						ExpireAfterDays: int32(14),
						IsLive:          true,
					},
				}
				p.CORS = &storage.CORSConfig{
					Origins: []string{
						"https://example.com",
						"https://example2.com",
					},
					ResponseHeaders: []string{
						"X-My-Header-2",
					},
					MaxAge: 3600,
				}
				p.CustomHostnames = []string{
					"my-bucket.example.com",
					"your-bucket.example.com",
				}
				return &p
			}(),
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters
				p.PublicRead = false
				p.PublicList = true
				p.Versioning = false
				p.Permissions = []*storage.BucketPermission{
					{
						Role: storage.BucketRoleReader,
						BucketUserRefs: []*meta.LocalReference{
							{Name: "user2"},
						},
					},
				}
				p.LifecyclePolicies = []*storage.BucketLifecyclePolicy{
					{
						Prefix:          "tmp/",
						ExpireAfterDays: int32(1),
						IsLive:          false,
					},
				}
				p.CORS = &storage.CORSConfig{
					Origins: []string{
						"https://example.com",
					},
					ResponseHeaders: nil,
					MaxAge:          1800,
				}
				p.CustomHostnames = nil
				return &p
			}(),
		},
	} {
		t.Run(name, func(t *testing.T) {
			project := "proj-" + t.Name()
			name := "bucket-" + t.Name()

			orig := &storage.Bucket{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: project,
				},
				Spec: storage.BucketSpec{
					ForProvider: *tc.origForProvider,
				},
			}

			apiClient, resolvedName, err := runBucketUpdateNamedWithFlags(
				t,
				name,
				tc.flags,
				test.WithDefaultProject(project),
				test.WithObjects(orig),
			)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, name, resolvedName)

			updated := &storage.Bucket{}
			require.NoError(t, apiClient.Get(
				t.Context(),
				api.NamespacedName(resolvedName, apiClient.Project),
				updated,
			))

			if tc.wantForProvider != nil {
				assert.Equal(t, *tc.wantForProvider, updated.Spec.ForProvider)
			}

			if tc.afterAssert != nil {
				tc.afterAssert(t, updated)
			}
		})
	}
}

func runBucketUpdateNamedWithFlags(
	t *testing.T,
	name string,
	flags []string,
	clientOpts ...test.ClientSetupOption,
) (*api.Client, string, error) {
	t.Helper()

	var cli struct {
		Bucket *bucketCmd `cmd:"" group:"storage.nine.ch" name:"bucket" help:"Update a Bucket."`
	}

	vars := create.BucketKongVars()
	maps.Copy(vars, BucketKongVars())

	return test.RunNamedWithFlags(
		t,
		&cli,
		vars,
		[]string{"bucket"}, // command path for UPDATE
		name,
		flags,
		func() (*string, *bool, *time.Duration) {
			if cli.Bucket == nil {
				return nil, nil, nil
			}
			return &cli.Bucket.Name, nil, nil
		},
		clientOpts...,
	)
}
