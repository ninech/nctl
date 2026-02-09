package delete

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
)

func TestCloudVM(t *testing.T) {
	t.Parallel()
	out := &bytes.Buffer{}
	cmd := cloudVMCmd{
		resourceCmd: resourceCmd{
			Writer:      format.NewWriter(out),
			Name:        "test",
			Force:       true,
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	cloudvm := test.CloudVirtualMachine("test", test.DefaultProject, "nine-es34", v1alpha1.VirtualMachinePowerState("on"))

	apiClient, err := test.SetupClient()
	if err != nil {
		t.Fatalf("failed to setup api client: %v", err)
	}

	ctx := t.Context()
	if err := apiClient.Create(ctx, cloudvm); err != nil {
		t.Fatalf("failed to create cloudvm: %v", err)
	}
	if err := apiClient.Get(ctx, api.ObjectName(cloudvm), cloudvm); err != nil {
		t.Fatalf("expected cloudvm to exist before deletion, got error: %v", err)
	}
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatalf("failed to run cloudvm delete command: %v", err)
	}
	err = apiClient.Get(ctx, api.ObjectName(cloudvm), cloudvm)
	if err == nil {
		t.Fatal("expected cloudvm to be deleted, but it still exists")
	}
	if !kerrors.IsNotFound(err) {
		t.Fatalf("expected cloudvm to be deleted (NotFound), but got error: %v", err)
	}

	if !strings.Contains(out.String(), "deletion started") {
		t.Errorf("expected output to contain 'deletion started', got %q", out.String())
	}
	if !strings.Contains(out.String(), cmd.Name) {
		t.Errorf("expected output to contain cloudvm name %q, got %q", cmd.Name, out.String())
	}
}
