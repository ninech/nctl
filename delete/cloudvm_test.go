package delete

import (
	"context"
	"testing"
	"time"

	"github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCloudVM(t *testing.T) {
	cmd := cloudVMCmd{
		resourceCmd: resourceCmd{
			Name:        "test",
			Force:       true,
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	cloudvm := test.CloudVirtualMachine("test", "default", "nine-es34", v1alpha1.VirtualMachinePowerState("on"))

	scheme, err := api.NewScheme()
	if err != nil {
		t.Fatal(err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	apiClient := &api.Client{WithWatch: client, Project: "default"}
	ctx := context.Background()

	if err := apiClient.Create(ctx, cloudvm); err != nil {
		t.Fatalf("cloudvm create error, got: %s", err)
	}
	if err := apiClient.Get(ctx, api.ObjectName(cloudvm), cloudvm); err != nil {
		t.Fatalf("expected cloudvm to exist, got: %s", err)
	}
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}
	err = apiClient.Get(ctx, api.ObjectName(cloudvm), cloudvm)
	if err == nil {
		t.Fatalf("expected cloudvm to be deleted, but exists")
	}
	if !errors.IsNotFound(err) {
		t.Fatalf("expected cloudvm to be deleted, got: %s", err.Error())
	}
}
