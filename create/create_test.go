package create

import (
	"context"
	"testing"
	"time"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	iam "github.com/ninech/apis/iam/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCreate(t *testing.T) {
	scheme, err := api.NewScheme()
	if err != nil {
		t.Fatal(err)
	}

	asa := &iam.APIServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: iam.APIServiceAccountSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      "test",
					Namespace: "default",
				},
			},
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	apiClient := &api.Client{WithWatch: client, Project: "default"}
	c := newCreator(apiClient, asa, "apiserviceaccount")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		t.Fatal(err)
	}

	// to test the wait we create a ticker that continously updates our
	// resource in a goroutine to simulate a controller doing the same
	ticker := time.NewTicker(100 * time.Millisecond)
	done := make(chan bool)
	errChan := make(chan error, 1)

	go func() {
		for {
			select {
			case <-done:
				close(errChan)
				return
			case <-ticker.C:
				if err := apiClient.Get(ctx, types.NamespacedName{Name: asa.Name, Namespace: asa.Namespace}, asa); err != nil {
					errChan <- err
				}

				asa.SetConditions(runtimev1.Available())
				if err := apiClient.Update(ctx, asa); err != nil {
					errChan <- err
				}
			}
		}
	}()

	resultFuncCalled := false
	if err := c.wait(ctx, waitStage{
		objectList: &iam.APIServiceAccountList{},
		onResult: func(event watch.Event) (bool, error) {
			resultFuncCalled = true
			return resourceAvailable(event)
		},
	}); err != nil {
		t.Fatal(err)
	}

	ticker.Stop()
	done <- true

	for err := range errChan {
		t.Fatal(err)
	}

	if !resultFuncCalled {
		t.Fatal("result func has not been called")
	}
}
