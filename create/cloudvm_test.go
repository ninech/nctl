package create

import (
	"context"
	"reflect"
	"testing"
	"time"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCloudVM(t *testing.T) {
	tests := []struct {
		name    string
		create  cloudVMCmd
		want    infrastructure.CloudVirtualMachineParameters
		wantErr bool
	}{
		{"simple", cloudVMCmd{}, infrastructure.CloudVirtualMachineParameters{}, false},
		{
			"disks",
			cloudVMCmd{Disks: map[string]string{"a": "1Gi"}},
			infrastructure.CloudVirtualMachineParameters{Disks: []infrastructure.Disk{{Name: "a", Size: resource.MustParse("1Gi")}}},
			false,
		},
		{
			"bootDisk",
			cloudVMCmd{BootDiskSize: "1Gi"},
			infrastructure.CloudVirtualMachineParameters{BootDisk: &infrastructure.Disk{Name: "root", Size: resource.MustParse("1Gi")}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.create.Name = "test-" + t.Name()
			tt.create.Wait = false
			tt.create.WaitTimeout = time.Second

			scheme, err := api.NewScheme()
			if err != nil {
				t.Fatal(err)
			}
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			apiClient := &api.Client{WithWatch: client, Project: "default"}
			ctx := context.Background()

			if err := tt.create.Run(ctx, apiClient); (err != nil) != tt.wantErr {
				t.Errorf("cloudVMCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			created := &infrastructure.CloudVirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: tt.create.Name, Namespace: apiClient.Project}}
			if err := apiClient.Get(ctx, api.ObjectName(created), created); (err != nil) != tt.wantErr {
				t.Fatalf("expected cloudVM to exist, got: %s", err)
			}
			if tt.wantErr {
				return
			}

			if !reflect.DeepEqual(created.Spec.ForProvider, tt.want) {
				t.Fatalf("expected CloudVirtualMachine.Spec.ForProvider = %v, got: %v", created.Spec.ForProvider, tt.want)
			}
		})
	}
}
