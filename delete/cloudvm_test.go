package delete

import (
	"context"
	"testing"
	"time"

	"github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
)

func TestCloudVM(t *testing.T) {
	ctx := context.Background()
	cmd := cloudVMCmd{
		resourceCmd: resourceCmd{
			Name:        "test",
			Force:       true,
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	cloudvm := test.CloudVirtualMachine("test", test.DefaultProject, "nine-es34", v1alpha1.VirtualMachinePowerState("on"))

	apiClient, err := test.SetupClient()
	require.NoError(t, err)

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
