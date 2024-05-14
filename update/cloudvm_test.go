package update

import (
	"context"
	"reflect"
	"testing"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCloudVM(t *testing.T) {
	tests := []struct {
		name    string
		create  infrastructure.CloudVirtualMachineParameters
		update  cloudVMCmd
		want    infrastructure.CloudVirtualMachineParameters
		wantErr bool
	}{
		{"simple", infrastructure.CloudVirtualMachineParameters{}, cloudVMCmd{}, infrastructure.CloudVirtualMachineParameters{}, false},
		{
			"hostname",
			infrastructure.CloudVirtualMachineParameters{},
			cloudVMCmd{Hostname: "a"},
			infrastructure.CloudVirtualMachineParameters{Hostname: "a"},
			false,
		},
		{
			"turn on",
			infrastructure.CloudVirtualMachineParameters{PowerState: infrastructure.VirtualMachinePowerState("off")},
			cloudVMCmd{On: ptr.To(bool(true))},
			infrastructure.CloudVirtualMachineParameters{PowerState: infrastructure.VirtualMachinePowerState("on")},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.update.Name = "test-" + t.Name()

			scheme, err := api.NewScheme()
			if err != nil {
				t.Fatal(err)
			}
			apiClient := &api.Client{WithWatch: fake.NewClientBuilder().WithScheme(scheme).Build(), Project: "default"}
			ctx := context.Background()

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
