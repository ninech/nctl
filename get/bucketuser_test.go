package get

import (
	"bytes"
	"strings"
	"testing"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/internal/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestBucketUser(t *testing.T) {
	t.Parallel()

	type buInstance struct {
		name     string
		project  string
		location meta.LocationName
	}
	tests := []struct {
		name          string
		instances     []buInstance
		get           bucketUserCmd
		out           outputFormat
		inAllProjects bool
		wantContain   []string
		wantLines     int
		wantErr       bool
	}{
		{
			name:        "simple",
			get:         bucketUserCmd{},
			out:         full,
			wantErr:     true,
			wantContain: []string{`no "BucketUsers" found`, `Project: default`},
		},
		{
			name: "single",
			instances: []buInstance{
				{
					name:     "test",
					project:  test.DefaultProject,
					location: meta.LocationNineES34,
				},
			},
			get:         bucketUserCmd{},
			out:         full,
			wantContain: []string{"nine-es34"},
			wantLines:   2, // header + result
		},
		{
			name: "multiple in one project",
			instances: []buInstance{
				{
					name:     "test1",
					project:  test.DefaultProject,
					location: meta.LocationNineES34,
				},
				{
					name:     "test2",
					project:  test.DefaultProject,
					location: meta.LocationNineES34,
				},
				{
					name:     "test3",
					project:  test.DefaultProject,
					location: meta.LocationNineCZ42,
				},
			},
			get:         bucketUserCmd{},
			out:         full,
			wantContain: []string{"nine-es34"},
			wantLines:   4,
		},
		{
			name: "not existing cloudVM",
			instances: []buInstance{
				{
					name:     "test",
					project:  test.DefaultProject,
					location: meta.LocationNineES34,
				},
			},
			get: bucketUserCmd{
				resourceCmd: resourceCmd{Name: "test2"},
			},
			out:     full,
			wantErr: true,
		},
		{
			name: "multiple in all projects",
			instances: []buInstance{
				{
					name:     "test",
					project:  test.DefaultProject,
					location: meta.LocationNineES34,
				},
				{
					name:     "dev",
					project:  "dev",
					location: meta.LocationNineES34,
				},
			},
			get:           bucketUserCmd{},
			out:           noHeader,
			inAllProjects: true,
			wantLines:     2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			objects := []client.Object{}
			for _, bu := range tt.instances {
				created := bucketUser(bu.name, bu.project, bu.location)
				objects = append(objects, created)
			}
			apiClient := test.SetupClient(t,
				test.WithProjectsFromResources(objects...),
				test.WithObjects(objects...),
				test.WithNameIndexFor(&storage.BucketUser{}),
				test.WithKubeconfig(),
			)
			buf := &bytes.Buffer{}
			cmd := NewTestCmd(buf, tt.out)
			cmd.AllProjects = tt.inAllProjects
			err := tt.get.Run(t.Context(), apiClient, cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("bucketUserCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
				t.Log(buf.String())
			}
			if tt.wantErr {
				for _, substr := range tt.wantContain {
					if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(substr)) {
						t.Errorf("bucketUserCmd.Run() error did not contain %q, err = %v", substr, err)
					}
				}
				return
			}

			for _, substr := range tt.wantContain {
				if !strings.Contains(buf.String(), substr) {
					t.Errorf("bucketUserCmd.Run() did not contain %q, out = %q", tt.wantContain, buf.String())
				}
			}
			if test.CountLines(buf.String()) != tt.wantLines {
				t.Errorf("expected the output to have %d lines, but found %d", tt.wantLines, test.CountLines(buf.String()))
			}
		})
	}
}

func bucketUser(name, project string, location meta.LocationName) *storage.BucketUser {
	return &storage.BucketUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		Spec: storage.BucketUserSpec{
			ForProvider: storage.BucketUserParameters{
				Location: meta.LocationName(location),
			},
		},
	}
}
