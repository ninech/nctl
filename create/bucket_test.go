package create

import (
	"testing"
	"time"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBucket(t *testing.T) {
	for name, tc := range map[string]struct {
		flags           []string
		wantForProvider *storage.BucketParameters
		ensureAfter     func(is *require.Assertions, b *storage.Bucket)
		wantErr         bool
	}{
		"create-minimal": {
			flags: []string{
				"--location=nine-es34",
			},
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters()
				p.Location = meta.LocationNineES34
				return &p
			}(),
		},
		"create-full": {
			flags: []string{
				"--location=nine-es34",
				"--public-read",
				"--public-list",
				"--versioning",
				"--permissions=reader=john",
				"--lifecycle-policy=prefix=logs/;expire-after-days=14;is-live=true",
				"--cors=origins=https://example.com;response-headers=X-My-Header;max-age=3600",
				"--custom-hostnames=my-bucket.example.com,your-bucket.example.com",
			},
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters()
				p.Location = meta.LocationNineES34
				p.PublicRead = true
				p.PublicList = true
				p.Versioning = true
				p.Permissions = []*storage.BucketPermission{
					{
						Role: storage.BucketRoleReader,
						BucketUserRefs: []*meta.LocalReference{
							{Name: "john"},
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
					},
					ResponseHeaders: []string{
						"X-My-Header",
					},
					MaxAge: 3600,
				}
				p.CustomHostnames = []string{
					"my-bucket.example.com",
					"your-bucket.example.com",
				}
				return &p
			}(),
		},
		"permissions-multiple-flags-merged": {
			flags: []string{
				"--location=nine-es34",
				"--permissions=reader=frontend,analytics",
				"--permissions=writer=ingest",
				"--permissions=reader=john",
			},
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters()
				p.Location = meta.LocationNineES34
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
		"permissions-single-flag-multiple-perms": {
			flags: []string{
				"--location=nine-es34",
				"--permissions=reader=frontend,analytics;writer=ingest;reader=john",
			},
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters()
				p.Location = meta.LocationNineES34
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
		"permissions-mixed-usage": {
			flags: []string{
				"--location=nine-es34",
				"--permissions=reader=frontend,analytics;writer=ingest;reader=john",
				"--permissions=reader=guest1,guest2",
			},
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters()
				p.Location = meta.LocationNineES34
				p.Permissions = []*storage.BucketPermission{
					{
						Role: storage.BucketRoleReader,
						BucketUserRefs: []*meta.LocalReference{
							{Name: "analytics"},
							{Name: "frontend"},
							{Name: "guest1"},
							{Name: "guest2"},
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
		"lifecycle-multiple-policies": {
			flags: []string{
				"--location=nine-cz42",
				"--lifecycle-policy=prefix=tmp/;expire-after-days=3;is-live=true",
				"--lifecycle-policy=prefix=archive/;expire-after-days=365;is-live=false",
			},
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters()
				p.Location = meta.LocationNineCZ42
				p.LifecyclePolicies = []*storage.BucketLifecyclePolicy{
					{
						Prefix:          "archive/",
						ExpireAfterDays: int32(365),
						IsLive:          false,
					},
					{
						Prefix:          "tmp/",
						ExpireAfterDays: int32(3),
						IsLive:          true,
					},
				}
				return &p
			}(),
		},
		"cors-duplicated-keys-in-one-flag": {
			flags: []string{
				"--location=nine-cz42",
				"--cors=origins=https://example.com;response-headers=X-My-Header;max-age=3600;origins=https://app.example.com;response-headers=ETag",
			},
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters()
				p.Location = meta.LocationNineCZ42
				p.CORS = &storage.CORSConfig{
					Origins: []string{
						"https://app.example.com",
						"https://example.com",
					},
					ResponseHeaders: []string{
						"ETag",
						"X-My-Header",
					},
					MaxAge: 3600,
				}
				return &p
			}(),
		},
		"cors-multiple-flags-merged": {
			flags: []string{
				"--location=nine-cz42",
				"--cors=origins=https://example.com;response-headers=ETag",
				"--cors=origins=https://app.example.com;response-headers=X-My-Header",
				"--cors=max-age=3600",
			},
			wantForProvider: func() *storage.BucketParameters {
				p := baseBucketParameters()
				p.Location = meta.LocationNineCZ42
				p.CORS = &storage.CORSConfig{
					Origins: []string{
						"https://app.example.com",
						"https://example.com",
					},
					ResponseHeaders: []string{
						"ETag",
						"X-My-Header",
					},
					MaxAge: 3600,
				}
				return &p
			}(),
		},
		"cors-conflicting-max-age": {
			flags: []string{
				"--location=nine-cz42",
				"--cors=max-age=3600",
				"--cors=max-age=1800",
			},
			wantErr: true,
		},
		"missing-required-flags": {
			flags:   []string{},
			wantErr: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			apiClient, resolvedName, err := runBucketCreateNamedWithFlags(t, "test-"+t.Name(), tc.flags)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			created := &storage.Bucket{}
			require.NoError(t, apiClient.Get(
				t.Context(),
				api.NamespacedName(resolvedName, apiClient.Project),
				created,
			))

			if tc.wantForProvider != nil {
				assert.Equal(t, *tc.wantForProvider, created.Spec.ForProvider)
			}
		})
	}
}

func runBucketCreateNamedWithFlags(
	t *testing.T,
	name string,
	flags []string,
	clientOpts ...test.ClientSetupOption,
) (*api.Client, string, error) {
	t.Helper()

	var cli struct {
		Bucket *bucketCmd `cmd:"" group:"storage.nine.ch" name:"bucket" help:"Create a new Bucket."`
	}

	return test.RunNamedWithFlags(
		t,
		&cli,
		BucketKongVars(),
		[]string{"bucket"}, // command path for CREATE
		name,
		flags,
		func() (*string, *bool, *time.Duration) {
			if cli.Bucket == nil {
				return nil, nil, nil
			}
			return &cli.Bucket.Name, &cli.Bucket.Wait, &cli.Bucket.WaitTimeout
		},
		clientOpts...,
	)
}
