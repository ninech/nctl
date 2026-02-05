package get

import (
	"bytes"
	"context"
	"strings"
	"testing"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCloudVM(t *testing.T) {
	type cvmInstance struct {
		name       string
		project    string
		powerState infrastructure.VirtualMachinePowerState
	}
	ctx := context.Background()
	tests := []struct {
		name          string
		instances     []cvmInstance
		get           cloudVMCmd
		out           outputFormat
		inAllProjects bool
		wantContain   []string
		wantLines     int
		wantErr       bool
	}{
		{
			name:        "simple",
			get:         cloudVMCmd{},
			out:         full,
			wantErr:     true,
			wantContain: []string{`no "CloudVirtualMachines" found`, `Project: default`},
		},
		{
			name: "single",
			instances: []cvmInstance{
				{
					name:       "test",
					project:    test.DefaultProject,
					powerState: infrastructure.VirtualMachinePowerState("on"),
				},
			},
			get:         cloudVMCmd{},
			out:         full,
			wantContain: []string{"on"},
			wantLines:   2, // header + result
		},
		{
			name: "multiple in one project",
			instances: []cvmInstance{
				{
					name:       "test1",
					project:    test.DefaultProject,
					powerState: infrastructure.VirtualMachinePowerState("on"),
				},
				{
					name:       "test2",
					project:    test.DefaultProject,
					powerState: infrastructure.VirtualMachinePowerState("off"),
				},
				{
					name:       "test3",
					project:    test.DefaultProject,
					powerState: infrastructure.VirtualMachinePowerState("shutdown"),
				},
			},
			get:         cloudVMCmd{},
			out:         full,
			wantContain: []string{"on", "off", "shutdown"},
			wantLines:   4,
		},
		{
			name: "not existing cloudVM",
			instances: []cvmInstance{
				{
					name:       "test",
					project:    test.DefaultProject,
					powerState: infrastructure.VirtualMachinePowerState("on"),
				},
			},
			get: cloudVMCmd{
				resourceCmd: resourceCmd{Name: "test2"},
			},
			out:     full,
			wantErr: true,
		},
		{
			name: "multiple in all projects",
			instances: []cvmInstance{
				{
					name:       "test",
					project:    test.DefaultProject,
					powerState: infrastructure.VirtualMachinePowerState("on"),
				},
				{
					name:       "dev",
					project:    "dev",
					powerState: infrastructure.VirtualMachinePowerState("on"),
				},
			},
			get:           cloudVMCmd{},
			out:           noHeader,
			inAllProjects: true,
			wantLines:     2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := []client.Object{}
			for _, cvm := range tt.instances {
				created := test.CloudVirtualMachine(cvm.name, cvm.project, "nine-es34", cvm.powerState)
				created.Status.AtProvider.PowerState = cvm.powerState
				objects = append(objects, created)
			}
			apiClient, err := test.SetupClient(
				test.WithProjectsFromResources(objects...),
				test.WithObjects(objects...),
				test.WithNameIndexFor(&infrastructure.CloudVirtualMachine{}),
				test.WithKubeconfig(t),
			)
			require.NoError(t, err)

			buf := &bytes.Buffer{}
			cmd := NewTestCmd(buf, tt.out)
			cmd.AllProjects = tt.inAllProjects
			err = tt.get.Run(ctx, apiClient, cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("cloudVMCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
				t.Log(buf.String())
			}
			if tt.wantErr {
				for _, substr := range tt.wantContain {
					if !strings.Contains(err.Error(), substr) {
						t.Errorf("cloudVMCmd.Run() error did not contain %q, err = %v", substr, err)
					}
				}
				return
			}

			for _, substr := range tt.wantContain {
				if !strings.Contains(buf.String(), substr) {
					t.Errorf("cloudVMCmd.Run() did not contain %q, out = %q", tt.wantContain, buf.String())
				}
			}
			if test.CountLines(buf.String()) != tt.wantLines {
				t.Errorf("expected the output to have %d lines, but found %d", tt.wantLines, test.CountLines(buf.String()))
			}
		})
	}
}
