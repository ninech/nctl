package delete

import (
	"context"
	"testing"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/ninech/nctl/internal/test"
)

func TestProjectConfig(t *testing.T) {
	namespace := "some-namespace"

	cfg := &apps.ProjectConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      namespace,
			Namespace: namespace,
		},
		Spec: apps.ProjectConfigSpec{},
	}

	cmd := configCmd{
		Force: true,
		Wait:  false,
	}

	apiClient, err := test.SetupClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	apiClient.Namespace = namespace

	ctx := context.Background()

	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}

	if !errors.IsNotFound(apiClient.Get(ctx, api.ObjectName(cfg), cfg)) {
		t.Fatalf("expected project configuration to not exist after delete, got %s", err)
	}
}
