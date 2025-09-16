package create

import (
	"context"
	"reflect"
	"testing"
	"time"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCloudVM(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		create  cloudVMCmd
		want    infrastructure.CloudVirtualMachineParameters
		wantErr bool
	}{
		{
			name: "simple",
		},
		{
			name: "disks",
			create: cloudVMCmd{
				Disks: map[string]string{"a": "1Gi"},
			},
			want: infrastructure.CloudVirtualMachineParameters{
				Disks: []infrastructure.Disk{
					{Name: "a", Size: resource.MustParse("1Gi")},
				},
			},
		},
		{
			name:   "bootDisk",
			create: cloudVMCmd{BootDiskSize: "1Gi"},
			want: infrastructure.CloudVirtualMachineParameters{
				BootDisk: &infrastructure.Disk{
					Name: "root", Size: resource.MustParse("1Gi"),
				},
			},
		},
		{
			name:   "reverseDNS",
			create: cloudVMCmd{ReverseDNS: "me.example.com"},
			want: infrastructure.CloudVirtualMachineParameters{
				ReverseDNS: "me.example.com",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.create.Name = "test-" + t.Name()
			tt.create.Wait = false
			tt.create.WaitTimeout = time.Second

			apiClient, err := test.SetupClient()
			require.NoError(t, err)

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
