package get

import (
	"bytes"
	"strings"
	"testing"

	observability "github.com/ninech/apis/observability/v1alpha1"
	"github.com/ninech/nctl/internal/test"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestGrafana(t *testing.T) {
	t.Parallel()

	type grafanaInstance struct {
		name              string
		project           string
		enableAdminAccess bool
	}

	tests := []struct {
		name          string
		instances     []grafanaInstance
		get           grafanaCmd
		out           outputFormat
		wantContain   []string
		wantLines     int
		inAllProjects bool
		wantErr       bool
	}{
		{
			name:        "no instances",
			wantErr:     true,
			wantContain: []string{`no "Grafanas" found`},
		},
		{
			name:        "single instance in project",
			instances:   []grafanaInstance{{name: "test", project: test.DefaultProject}},
			wantContain: []string{"test"},
			wantLines:   2, // header + result
		},
		{
			name: "multiple instances in one project",
			instances: []grafanaInstance{
				{name: "test1", project: test.DefaultProject},
				{name: "test2", project: test.DefaultProject},
				{name: "test3", project: test.DefaultProject},
			},
			wantContain: []string{"test1", "test2", "test3"},
			wantLines:   4, // header + result
		},
		{
			name: "multiple instances in multiple projects",
			instances: []grafanaInstance{
				{name: "test1", project: test.DefaultProject},
				{name: "test2", project: "dev"},
				{name: "test3", project: "testing"},
			},
			wantContain:   []string{"test1", "test2", "test3"},
			inAllProjects: true,
			wantLines:     4, // header + result
		},
		{
			name: "get by name",
			instances: []grafanaInstance{
				{name: "test1", project: test.DefaultProject},
				{name: "test2", project: test.DefaultProject},
			},
			get:         grafanaCmd{resourceCmd: resourceCmd{Name: "test1"}},
			wantContain: []string{"test1"},
			wantLines:   2,
		},
		{
			name: "admin access enabled",
			instances: []grafanaInstance{
				{name: "test1", project: test.DefaultProject, enableAdminAccess: true},
			},
			wantContain: []string{"test1", "ADMIN ACCESS", "true"},
			wantLines:   2,
		},
		{
			name: "admin access disabled",
			instances: []grafanaInstance{
				{name: "test1", project: test.DefaultProject, enableAdminAccess: false},
			},
			wantContain: []string{"test1", "ADMIN ACCESS", "false"},
			wantLines:   2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			objects := []client.Object{}
			for _, instance := range tt.instances {
				g := test.Grafana(instance.name, instance.project)
				g.Spec.ForProvider.EnableAdminAccess = instance.enableAdminAccess
				objects = append(objects, g)
			}
			apiClient := test.SetupClient(t,
				test.WithProjectsFromResources(objects...),
				test.WithObjects(objects...),
				test.WithNameIndexFor(&observability.Grafana{}),
				test.WithKubeconfig(),
			)

			if tt.out == "" {
				tt.out = full
			}
			buf := &bytes.Buffer{}
			cmd := NewTestCmd(buf, tt.out)
			cmd.AllProjects = tt.inAllProjects
			err := tt.get.Run(t.Context(), apiClient, cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("grafanaCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				for _, substr := range tt.wantContain {
					if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(substr)) {
						t.Errorf("grafanaCmd.Run() error did not contain %q, err = %v", substr, err)
					}
				}
				return
			}

			for _, substr := range tt.wantContain {
				if !strings.Contains(buf.String(), substr) {
					t.Errorf("grafanaCmd.Run() did not contain %q, out = %q", substr, buf.String())
				}
			}
			if test.CountLines(buf.String()) != tt.wantLines {
				t.Errorf("expected the output to have %d lines, but found %d", tt.wantLines, test.CountLines(buf.String()))
				t.Log(buf.String())
			}
		})
	}
}
