package get

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestBucketGet(t *testing.T) {
	type bucketInstance struct {
		name     string
		project  string
		location meta.LocationName
		spec     func(*storage.BucketSpec)
		status   func(*storage.BucketStatus)
	}

	ctx := context.Background()

	tests := []struct {
		name          string
		instances     []bucketInstance
		getCmd        bucketCmd
		out           outputFormat
		inAllProjects bool
		wantContain   []string
		wantLines     int
		wantErr       bool
	}{
		{
			name:        "no buckets",
			getCmd:      bucketCmd{},
			out:         full,
			wantErr:     true,
			wantContain: []string{`no "Buckets" found`, `Project: default`},
		},
		{
			name: "list single",
			instances: []bucketInstance{
				{name: "b1", project: test.DefaultProject, location: meta.LocationNineES34},
			},
			getCmd:      bucketCmd{},
			out:         full,
			wantContain: []string{"NAME", "LOCATION", "b1", "nine-es34"},
			wantLines:   2, // header + 1 row
		},
		{
			name: "list multiple (noHeader)",
			instances: []bucketInstance{
				{name: "a", project: test.DefaultProject, location: meta.LocationNineES34},
				{name: "b", project: test.DefaultProject, location: meta.LocationNineCZ42},
			},
			getCmd:      bucketCmd{},
			out:         noHeader,
			wantContain: []string{"a", "b"},
			wantLines:   2,
		},
		{
			name: "permissions empty",
			instances: []bucketInstance{
				{name: "b1", project: test.DefaultProject, location: meta.LocationNineES34},
			},
			getCmd: bucketCmd{
				resourceCmd:      resourceCmd{Name: "b1"},
				PrintPermissions: true,
			},
			out:         full,
			wantContain: []string{`No permissions defined for bucket "b1"`},
			wantLines:   1,
		},
		{
			name: "permissions with roles",
			instances: []bucketInstance{
				{
					name:     "b1",
					project:  test.DefaultProject,
					location: meta.LocationNineES34,
					spec: func(s *storage.BucketSpec) {
						s.ForProvider.Permissions = []*storage.BucketPermission{
							{Role: storage.BucketRoleReader, BucketUserRefs: []*meta.LocalReference{{Name: "alice"}, {Name: "bob"}}},
							{Role: storage.BucketRoleWriter, BucketUserRefs: []*meta.LocalReference{{Name: "deploy"}}},
						}
					},
				},
			},
			getCmd: bucketCmd{
				resourceCmd:      resourceCmd{Name: "b1"},
				PrintPermissions: true,
			},
			out:         full,
			wantContain: []string{"reader", "alice,bob", "writer", "deploy"},
			wantLines:   3, // header + 2 rows
		},
		{
			name: "lifecycle empty",
			instances: []bucketInstance{
				{name: "b1", project: test.DefaultProject, location: meta.LocationNineES34},
			},
			getCmd: bucketCmd{
				resourceCmd:            resourceCmd{Name: "b1"},
				PrintLifecyclePolicies: true,
			},
			out:         full,
			wantContain: []string{`No lifecycle policies defined for bucket "b1"`},
			wantLines:   1,
		},
		{
			name: "lifecycle one rule (days)",
			instances: []bucketInstance{
				{
					name:     "b1",
					project:  test.DefaultProject,
					location: meta.LocationNineES34,
					spec: func(s *storage.BucketSpec) {
						s.ForProvider.LifecyclePolicies = []*storage.BucketLifecyclePolicy{
							{Prefix: "p/", ExpireAfterDays: 7, IsLive: true},
						}
					},
				},
			},
			getCmd: bucketCmd{
				resourceCmd:            resourceCmd{Name: "b1"},
				PrintLifecyclePolicies: true,
			},
			out:         full,
			wantContain: []string{"PREFIX", "EXPIRE AFTER", "IS LIVE", "p/", "7d", "true"},
			wantLines:   2,
		},
		{
			name: "cors nil",
			instances: []bucketInstance{
				{name: "b1", project: test.DefaultProject, location: meta.LocationNineES34},
			},
			getCmd: bucketCmd{
				resourceCmd: resourceCmd{Name: "b1"},
				PrintCORS:   true,
			},
			out:         full,
			wantContain: []string{`No CORS configuration defined for bucket "b1"`},
			wantLines:   1,
		},
		{
			name: "cors present (empty origins still visible)",
			instances: []bucketInstance{
				{
					name:     "b1",
					project:  test.DefaultProject,
					location: meta.LocationNineES34,
					spec: func(s *storage.BucketSpec) {
						s.ForProvider.CORS = &storage.CORSConfig{
							Origins:         []string{}, // explicitly empty: show "-"
							ResponseHeaders: []string{"ETag"},
							MaxAge:          7200,
						}
					},
				},
			},
			getCmd: bucketCmd{
				resourceCmd: resourceCmd{Name: "b1"},
				PrintCORS:   true,
			},
			out:         full,
			wantContain: []string{"ORIGINS", "RESPONSE HEADERS", "MAX-AGE (s)", "-", "ETag", "7200"},
			wantLines:   2,
		},
		{
			name: "custom hostnames none",
			instances: []bucketInstance{
				{name: "b1", project: test.DefaultProject, location: meta.LocationNineES34},
			},
			getCmd: bucketCmd{
				resourceCmd:          resourceCmd{Name: "b1"},
				PrintCustomHostnames: true,
			},
			out:         full,
			wantContain: []string{`No custom hostnames defined for bucket "b1"`},
			wantLines:   1,
		},
		{
			name: "custom hostnames pending + verified + error (detailed)",
			instances: []bucketInstance{
				{
					name:     "b1",
					project:  test.DefaultProject,
					location: meta.LocationNineES34,
					spec: func(s *storage.BucketSpec) {
						s.ForProvider.CustomHostnames = []string{"cdn.example.com", "img.example.com", "pending.example.com"}
					},
					status: func(st *storage.BucketStatus) {
						now := metav1.NewTime(time.Now())
						st.AtProvider.CustomHostnamesVerification = meta.DNSVerificationStatus{
							CNAMETarget:    "bucket.s3.nine.ch",
							TXTRecordValue: "_nine-verification=abc123",
							StatusEntries: meta.DNSVerificationStatusEntries{
								// verified CNAME for cdn.example.com
								{Name: "cdn.example.com", CheckType: meta.DNSCheckCNAME, LatestSuccess: &now},
								// failing TXT for img.example.com
								{Name: "img.example.com", CheckType: meta.DNSCheckTXT, Error: &meta.VerificationError{Message: "NXDOMAIN", Timestamp: now}},
								// pending.example.com has no entries -> pending row
							},
						}
					},
				},
			},
			getCmd: bucketCmd{
				resourceCmd:          resourceCmd{Name: "b1"},
				PrintCustomHostnames: true,
			},
			out: full,
			wantContain: []string{
				"HOSTNAME", "CHECK TYPE", "EXPECTED", "VERIFIED", "LAST SUCCESS", "ERROR",
				"cdn.example.com", "CNAME", "bucket.s3.nine.ch", "true",
				"img.example.com", "TXT", "_nine-verification=abc123", "false", "NXDOMAIN",
				"pending.example.com", "pending",
			},
			// header + up to 3 rows (CNAME verified, TXT error, pending line)
			wantLines: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objs []client.Object
			for _, bi := range tt.instances {
				b := bucket(bi.name, bi.project, bi.location)
				if bi.spec != nil {
					bi.spec(&b.Spec)
				}
				if bi.status != nil {
					bi.status(&b.Status)
				}
				objs = append(objs, &b)
			}

			apiClient, err := test.SetupClient(
				test.WithProjectsFromResources(objs...),
				test.WithObjects(objs...),
				test.WithNameIndexFor(&storage.Bucket{}),
				test.WithKubeconfig(t),
			)
			require.NoError(t, err)

			buf := &bytes.Buffer{}
			cmd := tt.getCmd
			get := NewTestCmd(buf, tt.out)
			get.AllProjects = tt.inAllProjects
			err = cmd.Run(ctx, apiClient, get)

			if tt.wantErr {
				require.Error(t, err)
				for _, s := range tt.wantContain {
					assert.Contains(t, strings.ToLower(err.Error()), strings.ToLower(s), "missing expected substring %q in error:\n%s", s, err)
				}
				return
			}
			require.NoError(t, err, buf.String())

			outStr := buf.String()
			for _, s := range tt.wantContain {
				assert.Contains(t, outStr, s, "missing expected substring %q in output:\n%s", s, outStr)
			}
			if tt.wantLines > 0 {
				assert.Equal(t, tt.wantLines, test.CountLines(outStr), "unexpected number of lines:\n%s", outStr)
			}
		})
	}
}

func bucket(name, project string, loc meta.LocationName) storage.Bucket {
	return storage.Bucket{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		Spec: storage.BucketSpec{
			ForProvider: storage.BucketParameters{
				Location:    loc,
				StorageType: "standard",
			},
		},
	}
}
