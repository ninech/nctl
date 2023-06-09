package delete

import (
	"context"
	"testing"

	apps "github.com/ninech/apis/apps/v1alpha1"
	iam "github.com/ninech/apis/iam/v1alpha1"
	"github.com/ninech/nctl/api"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDeleter(t *testing.T) {
	asa := &iam.APIServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: iam.APIServiceAccountSpec{},
	}

	scheme, err := api.NewScheme()
	if err != nil {
		t.Fatal(err)
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(asa).Build()
	apiClient := &api.Client{WithWatch: client, Namespace: "default"}
	ctx := context.Background()
	d := newDeleter(asa, iam.APIServiceAccountKind)

	if err := d.deleteResource(ctx, apiClient, 0, false, true); err != nil {
		t.Fatalf("error while deleting %s: %s", apps.ApplicationKind, err)
	}

	if !errors.IsNotFound(apiClient.Get(ctx, api.ObjectName(asa), asa)) {
		t.Fatalf("expected resource to not exist after delete, got %s", err)
	}
}
