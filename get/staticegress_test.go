package get

import (
	"bytes"
	"strings"
	"testing"

	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/internal/test"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestStaticEgress(t *testing.T) {
	t.Parallel()

	type staticEgressInstance struct {
		name     string
		project  string
		appName  string
		disabled bool
	}

	tests := []struct {
		name          string
		instances     []staticEgressInstance
		get           staticEgressCmd
		out           outputFormat
		wantContain   []string
		wantLines     int
		inAllProjects bool
		wantErr       bool
	}{
		{
			name:        "no instances",
			wantErr:     true,
			wantContain: []string{`no "StaticEgresses" found`},
		},
		{
			name:        "single instance in project",
			instances:   []staticEgressInstance{{name: "test", project: test.DefaultProject, appName: "my-app"}},
			wantContain: []string{"test", "my-app"},
			wantLines:   2, // header + result
		},
		{
			name: "multiple instances in one project",
			instances: []staticEgressInstance{
				{name: "test1", project: test.DefaultProject, appName: "app1"},
				{name: "test2", project: test.DefaultProject, appName: "app2"},
				{name: "test3", project: test.DefaultProject, appName: "app3"},
			},
			wantContain: []string{"test1", "test2", "test3"},
			wantLines:   4, // header + results
		},
		{
			name: "multiple instances in multiple projects",
			instances: []staticEgressInstance{
				{name: "test1", project: test.DefaultProject, appName: "app1"},
				{name: "test2", project: "dev", appName: "app2"},
				{name: "test3", project: "testing", appName: "app3"},
			},
			wantContain:   []string{"test1", "test2", "test3"},
			inAllProjects: true,
			wantLines:     4, // header + results
		},
		{
			name: "get by name",
			instances: []staticEgressInstance{
				{name: "test1", project: test.DefaultProject, appName: "app1"},
				{name: "test2", project: test.DefaultProject, appName: "app2"},
			},
			get:         staticEgressCmd{resourceCmd: resourceCmd{Name: "test1"}},
			wantContain: []string{"test1"},
			wantLines:   2,
		},
		{
			name: "disabled shown",
			instances: []staticEgressInstance{
				{name: "test1", project: test.DefaultProject, appName: "app1", disabled: true},
			},
			wantContain: []string{"test1", "DISABLED", "true"},
			wantLines:   2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			objects := []client.Object{}
			for _, instance := range tt.instances {
				se := test.StaticEgress(instance.name, instance.project, instance.appName)
				se.Spec.ForProvider.Disabled = instance.disabled
				objects = append(objects, se)
			}
			apiClient := test.SetupClient(t,
				test.WithProjectsFromResources(objects...),
				test.WithObjects(objects...),
				test.WithNameIndexFor(&networking.StaticEgress{}),
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
				t.Errorf("staticEgressCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				for _, substr := range tt.wantContain {
					if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(substr)) {
						t.Errorf("staticEgressCmd.Run() error did not contain %q, err = %v", substr, err)
					}
				}
				return
			}

			for _, substr := range tt.wantContain {
				if !strings.Contains(buf.String(), substr) {
					t.Errorf("staticEgressCmd.Run() did not contain %q, out = %q", substr, buf.String())
				}
			}
			if test.CountLines(buf.String()) != tt.wantLines {
				t.Errorf("expected the output to have %d lines, but found %d", tt.wantLines, test.CountLines(buf.String()))
				t.Log(buf.String())
			}
		})
	}
}
