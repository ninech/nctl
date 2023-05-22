package get

import (
	"context"
	"testing"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestApplication(t *testing.T) {
	app := apps.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: apps.ApplicationSpec{},
	}
	app2 := app
	app2.Name = app2.Name + "-2"

	cmd := applicationsCmd{
		// this actually does nothing in this test as the fake client does not
		// support the MatchingFields list option.
		Name: app.Name,
	}

	get := &Cmd{
		Output: "full",
	}

	scheme, err := api.NewScheme()
	if err != nil {
		t.Fatal(err)
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&app, &app2).Build()
	apiClient := &api.Client{WithWatch: client, Namespace: "default"}
	ctx := context.Background()

	// TODO: verify command output
	if err := cmd.Run(ctx, apiClient, get); err != nil {
		t.Fatal(err)
	}
}
