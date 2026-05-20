package update

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestCloudVM(t *testing.T) {
	t.Parallel()

	noFlagsInterceptor := &interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			oldRV := obj.GetResourceVersion()
			if err := c.Update(ctx, obj, opts...); err != nil {
				return err
			}
			obj.SetResourceVersion(oldRV)
			return nil
		},
	}

	tests := []struct {
		name             string
		create           infrastructure.CloudVirtualMachineParameters
		update           cloudVMCmd
		want             infrastructure.CloudVirtualMachineParameters
		wantErr          bool
		interceptorFuncs *interceptor.Funcs
	}{
		{
			name:             "no-flags",
			wantErr:          true,
			interceptorFuncs: noFlagsInterceptor,
		},
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
			update: cloudVMCmd{On: new(bool(true))},
			want: infrastructure.CloudVirtualMachineParameters{
				PowerState: infrastructure.VirtualMachinePowerState("on"),
			},
		},
		{
			name: "set reverse DNS",
			create: infrastructure.CloudVirtualMachineParameters{
				ReverseDNS: "",
			},
			update: cloudVMCmd{ReverseDNS: "me.example.com"},
			want: infrastructure.CloudVirtualMachineParameters{
				ReverseDNS: "me.example.com",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := &bytes.Buffer{}
			tt.update.Writer = format.NewWriter(out)
			tt.update.Name = "test-" + t.Name()

			var opts []test.ClientSetupOption
			if tt.interceptorFuncs != nil {
				opts = append(opts, test.WithInterceptorFuncs(*tt.interceptorFuncs))
			}
			apiClient := test.SetupClient(t, opts...)

			created := test.CloudVirtualMachine(tt.update.Name, apiClient.Project, "nine-es34", tt.create.PowerState)
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(t.Context(), created); err != nil {
				t.Fatalf("cloudvm create error, got: %s", err)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), created); err != nil {
				t.Fatalf("expected cloudvm to exist, got: %s", err)
			}

			updated := &infrastructure.CloudVirtualMachine{ObjectMeta: metav1.ObjectMeta{Name: created.Name, Namespace: created.Namespace}}
			if err := tt.update.Run(t.Context(), apiClient); (err != nil) != tt.wantErr {
				t.Errorf("cloudVMCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(updated), updated); err != nil {
				t.Fatalf("expected cloudvm to exist, got: %s", err)
			}

			if !reflect.DeepEqual(updated.Spec.ForProvider, tt.want) {
				t.Fatalf("expected CloudVirtualMachine.Spec.ForProvider = %v, got: %v", updated.Spec.ForProvider, tt.want)
			}

			if !tt.wantErr {
				if !strings.Contains(out.String(), "updated") {
					t.Errorf("expected output to contain 'updated', got: %s", out.String())
				}
				if !strings.Contains(out.String(), tt.update.Name) {
					t.Errorf("expected output to contain %q, got: %s", tt.update.Name, out.String())
				}
			}
		})
	}
}
