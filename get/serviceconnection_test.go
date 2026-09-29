package get

import (
	"bytes"
	"strings"
	"testing"

	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/internal/testutil"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestServiceConnection(t *testing.T) {
	t.Parallel()

	type serviceConnectionInstance struct {
		name        string
		project     string
		source      string
		destination string
	}

	tests := []struct {
		name      string
		instances []serviceConnectionInstance
		get       serviceConnectionCmd
		// out defines the output format and will bet set to "full" if
		// not given
		out           outputFormat
		wantContain   []string
		wantLines     int
		inAllProjects bool
		wantErr       bool
	}{
		{
			name:        "simple",
			wantErr:     true,
			wantContain: []string{`no "ServiceConnections" found`},
		},
		{
			name: "single instance in project",
			instances: []serviceConnectionInstance{
				{
					name:        "testConnection",
					project:     testutil.DefaultProject,
					source:      "test-source-1",
					destination: "test-destination-1",
				},
			},
			wantContain: []string{"test-source-1", "test-destination-1"},
			wantLines:   2, // header + result
		},
		{
			name: "multiple instances in one project",
			instances: []serviceConnectionInstance{
				{
					name:        "testConnection1",
					project:     testutil.DefaultProject,
					source:      "test-source-1",
					destination: "test-destination-1",
				},
				{
					name:        "test2",
					project:     testutil.DefaultProject,
					source:      "test-source-2",
					destination: "test-destination-2",
				},
				{
					name:        "test3",
					project:     testutil.DefaultProject,
					source:      "test-source-3",
					destination: "test-destination-3",
				},
			},
			wantContain: []string{
				"test-source-1", "test-destination-1",
				"test-source-2", "test-destination-2",
				"test-source-3", "test-destination-3",
			},
			wantLines: 4, // header + result
		},
		{
			name: "get-by-name",
			instances: []serviceConnectionInstance{
				{
					name:        "test1",
					project:     testutil.DefaultProject,
					source:      "test-source-1",
					destination: "test-destination-1",
				},
				{
					name:        "test2",
					project:     testutil.DefaultProject,
					source:      "test-source-2",
					destination: "test-destination-2",
				},
			},
			get:         serviceConnectionCmd{ResourceCmd: ResourceCmd{Name: "test1"}},
			wantContain: []string{"test1", "test-source-1", "test-destination-1"},
			wantLines:   2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			objects := []client.Object{}
			for _, instance := range tt.instances {
				created := testutil.ServiceConnection(instance.name, instance.project)
				created.Spec.ForProvider.Source.Reference.Name = instance.source
				created.Spec.ForProvider.Destination.Name = instance.destination
				objects = append(objects, created)
			}

			apiClient := testutil.SetupClient(t,
				testutil.WithProjectsFromResources(objects...),
				testutil.WithObjects(objects...),
				testutil.WithNameIndexFor(&networking.ServiceConnection{}),
				testutil.WithKubeconfig(),
			)
			if tt.out == "" {
				tt.out = full
			}
			buf := &bytes.Buffer{}
			cmd := NewTestCmd(buf, tt.out)
			cmd.AllProjects = tt.inAllProjects
			err := tt.get.Run(t.Context(), apiClient, cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("serviceConnectionCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				for _, substr := range tt.wantContain {
					if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(substr)) {
						t.Errorf("serviceConnectionCmd.Run() error did not contain %q, err = %v", substr, err)
					}
				}
				return
			}

			for _, substr := range tt.wantContain {
				if !strings.Contains(buf.String(), substr) {
					t.Errorf("serviceConnectionCmd.Run() did not contain %q, out = %q", tt.wantContain, buf.String())
				}
			}
			if testutil.CountLines(buf.String()) != tt.wantLines {
				t.Errorf("expected the output to have %d lines, but found %d", tt.wantLines, testutil.CountLines(buf.String()))
				t.Log(buf.String())
			}
		})
	}
}
