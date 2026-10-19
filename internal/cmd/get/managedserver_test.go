package get

import (
	"bytes"
	"strings"
	"testing"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/internal/testutil"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestManagedServer(t *testing.T) {
	t.Parallel()

	type server struct {
		name     string
		project  string
		platform infrastructure.ManagedServerPlatform
	}
	tests := []struct {
		name          string
		servers       []server
		get           managedServerCmd
		out           outputFormat
		inAllProjects bool
		wantContain   []string
		wantLines     int
		wantErr       bool
	}{
		{
			name:        "none",
			out:         full,
			wantErr:     true,
			wantContain: []string{`no "ManagedServers" found`, `Project: default`},
		},
		{
			name:        "single",
			servers:     []server{{name: "test", project: testutil.DefaultProject, platform: "dedicated"}},
			out:         full,
			wantContain: []string{"test", "dedicated", "ubuntu24.04", "192.0.2.1,2001:db8::1"},
			wantLines:   2,
		},
		{
			name: "multiple in one project",
			servers: []server{
				{name: "test1", project: testutil.DefaultProject, platform: "dedicated"},
				{name: "test2", project: testutil.DefaultProject, platform: "virtual-v1"},
				{name: "test3", project: testutil.DefaultProject, platform: "virtual-v2"},
			},
			out:         full,
			wantContain: []string{"dedicated", "virtual-v1", "virtual-v2"},
			wantLines:   4,
		},
		{
			name:    "not existing",
			servers: []server{{name: "test", project: testutil.DefaultProject, platform: "dedicated"}},
			get:     managedServerCmd{ResourceCmd: ResourceCmd{Name: "test2"}},
			out:     full,
			wantErr: true,
		},
		{
			name: "multiple in all projects",
			servers: []server{
				{name: "test", project: testutil.DefaultProject, platform: "dedicated"},
				{name: "dev", project: "dev", platform: "dedicated"},
			},
			out:           noHeader,
			inAllProjects: true,
			wantLines:     2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			objects := []client.Object{}
			for _, s := range tt.servers {
				objects = append(objects, testutil.ManagedServer(s.name, s.project, "nine-es34", s.platform))
			}
			apiClient := testutil.SetupClient(t,
				testutil.WithProjectsFromResources(objects...),
				testutil.WithObjects(objects...),
				testutil.WithNameIndexFor(&infrastructure.ManagedServer{}),
				testutil.WithKubeconfig(),
			)
			buf := &bytes.Buffer{}
			cmd := NewTestCmd(t, buf, tt.out)
			cmd.AllProjects = tt.inAllProjects
			err := tt.get.Run(t.Context(), apiClient, cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("managedServerCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
				t.Log(buf.String())
			}
			if tt.wantErr {
				for _, substr := range tt.wantContain {
					if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(substr)) {
						t.Errorf("managedServerCmd.Run() error did not contain %q, err = %v", substr, err)
					}
				}
				return
			}

			for _, substr := range tt.wantContain {
				if !strings.Contains(buf.String(), substr) {
					t.Errorf("managedServerCmd.Run() did not contain %q, out = %q", substr, buf.String())
				}
			}
			if testutil.CountLines(buf.String()) != tt.wantLines {
				t.Errorf("expected the output to have %d lines, but found %d", tt.wantLines, testutil.CountLines(buf.String()))
			}
		})
	}
}
