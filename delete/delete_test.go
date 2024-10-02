package delete

import (
	"context"
	"testing"

	apps "github.com/ninech/apis/apps/v1alpha1"
	iam "github.com/ninech/apis/iam/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeleter(t *testing.T) {
	ctx := context.Background()
	asa := &iam.APIServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: test.DefaultProject,
		},
		Spec: iam.APIServiceAccountSpec{},
	}
	apiClient, err := test.SetupClient(
		test.WithObjects(asa),
	)
	require.NoError(t, err)

	d := newDeleter(asa, iam.APIServiceAccountKind)

	if err := d.deleteResource(ctx, apiClient, 0, false, true); err != nil {
		t.Fatalf("error while deleting %s: %s", apps.ApplicationKind, err)
	}

	if !errors.IsNotFound(apiClient.Get(ctx, api.ObjectName(asa), asa)) {
		t.Fatalf("expected resource to not exist after delete, got %s", err)
	}
}
