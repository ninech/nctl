package create

import (
	"context"
	"testing"
	"time"

	"github.com/ninech/nctl/api"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestVCluster(t *testing.T) {
	scheme, err := api.NewScheme()
	if err != nil {
		t.Fatal(err)
	}

	cmd := vclusterCmd{
		resourceCmd: resourceCmd{
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	cluster := cmd.newCluster("default")
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()
	apiClient := &api.Client{WithWatch: client, Project: "default"}
	ctx := context.Background()

	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}

	if err := apiClient.Get(ctx, api.ObjectName(cluster), cluster); err != nil {
		t.Fatalf("expected vcluster to exist, got: %s", err)
	}
}
