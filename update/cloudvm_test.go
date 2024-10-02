package update

import (
	"context"
	"reflect"
	"testing"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestCloudVM(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		create  infrastructure.CloudVirtualMachineParameters
		update  cloudVMCmd
		want    infrastructure.CloudVirtualMachineParameters
		wantErr bool
	}{
		{
			name: "simple",
		},
		{
			name:   "hostname",
			update: cloudVMCmd{Hostname: "a"},
			want:   infrastructure.CloudVirtualMachineParameters{Hostname: "a"},
		},
		{
			name: "turn on",
			create: infrastructure.CloudVirtualMachineParameters{
				PowerState: infrastructure.VirtualMachinePowerState("off"),
			},
			update: cloudVMCmd{On: ptr.To(bool(true))},
			want: infrastructure.CloudVirtualMachineParameters{
				PowerState: infrastructure.VirtualMachinePowerState("on"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.update.Name = "test-" + t.Name()

			apiClient, err := test.SetupClient()
			require.NoError(t, err)

			created := test.CloudVirtualMachine(tt.update.Name, apiClient.Project, "nine-es34", tt.create.PowerState)
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(ctx, created); err != nil {
				t.Fatalf("cloudvm create error, got: %s", err)
			}
			if err := apiClient.Get(ctx, api.ObjectName(created), created); err != nil {
				t.Fatalf("expected cloudvm to exist, got: %s", err)
			}

			updated := &infrastructure.CloudVirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: created.Name, Namespace: created.Namespace}}
			if err := tt.update.Run(ctx, apiClient); (err != nil) != tt.wantErr {
				t.Errorf("cloudVMCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := apiClient.Get(ctx, api.ObjectName(updated), updated); err != nil {
				t.Fatalf("expected cloudvm to exist, got: %s", err)
			}

			if !reflect.DeepEqual(updated.Spec.ForProvider, tt.want) {
				t.Fatalf("expected CloudVirtualMachine.Spec.ForProvider = %v, got: %v", updated.Spec.ForProvider, tt.want)
			}
		})
	}
}
