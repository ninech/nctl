package get

import (
	"bytes"
	"context"
	"strings"
	"testing"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCloudVM(t *testing.T) {
	tests := []struct {
		name        string
		instances   map[string]infrastructure.CloudVirtualMachineParameters
		get         cloudVMCmd
		out         output
		wantContain []string
		wantErr     bool
	}{
		{"simple", map[string]infrastructure.CloudVirtualMachineParameters{}, cloudVMCmd{}, full, []string{"no CloudVirtualMachines found in project default\n"}, false},
		{
			"single",
			map[string]infrastructure.CloudVirtualMachineParameters{"test": {PowerState: infrastructure.VirtualMachinePowerState("on")}},
			cloudVMCmd{},
			full,
			[]string{"on"},
			false,
		},
		{
			"multiple",
			map[string]infrastructure.CloudVirtualMachineParameters{
				"test1": {PowerState: infrastructure.VirtualMachinePowerState("on")},
				"test2": {PowerState: infrastructure.VirtualMachinePowerState("off")},
				"test3": {PowerState: infrastructure.VirtualMachinePowerState("shutdown")},
			},
			cloudVMCmd{},
			full,
			[]string{"on", "off", "shutdown"},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			tt.get.out = buf

			scheme, err := api.NewScheme()
			if err != nil {
				t.Fatal(err)
			}

			objects := []client.Object{}
			for name, instance := range tt.instances {
				created := test.CloudVirtualMachine(name, "default", "nine-es34", instance.PowerState)
				created.Spec.ForProvider = instance
				created.Status.AtProvider.PowerState = instance.PowerState
				objects = append(objects, created)
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithIndex(&infrastructure.CloudVirtualMachine{}, "metadata.name", func(o client.Object) []string {
					return []string{o.GetName()}
				}).
				WithObjects(objects...).Build()
			apiClient := &api.Client{WithWatch: client, Project: "default"}
			ctx := context.Background()

			if err := tt.get.Run(ctx, apiClient, &Cmd{Output: tt.out}); (err != nil) != tt.wantErr {
				t.Errorf("cloudVMCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			for _, substr := range tt.wantContain {
				if !strings.Contains(buf.String(), substr) {
					t.Errorf("cloudVMCmd.Run() did not contain %q, out = %q", tt.wantContain, buf.String())
				}
			}
		})
	}
}
