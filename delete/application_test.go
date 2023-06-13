package delete

import (
	"context"
	"testing"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestApplication(t *testing.T) {
	app := &apps.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: apps.ApplicationSpec{},
	}

	cmd := applicationCmd{
		Force: true,
		Wait:  false,
		Name:  app.Name,
	}

	scheme, err := api.NewScheme()
	if err != nil {
		t.Fatal(err)
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()
	apiClient := &api.Client{WithWatch: client, Project: "default"}

	ctx := context.Background()
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}

	if !errors.IsNotFound(apiClient.Get(ctx, api.ObjectName(app), app)) {
		t.Fatalf("expected application to not exist after delete, got %s", err)
	}
}
