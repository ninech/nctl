package api

import (
	"context"
	"fmt"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// RotateCredentials makes the controller of mg reset its credentials.
// It sets [RotateCredentialsAnnotation] on mg to the current time,
// like kubectl rollout restart does,
// and returns the value it set.
// The controller stores that value in status.atProvider.lastCredentialsRotation
// once it has reset the credentials,
// and clears status.atProvider.resettingCredentials
// once it has applied the new ones.
func (c *Client) RotateCredentials(ctx context.Context, mg resource.Managed) (string, error) {
	requested := time.Now().UTC().Truncate(time.Second)
	// The controller ignores a value it has reset the credentials for already,
	// which happens for two resets within the same second.
	if mg.GetAnnotations()[RotateCredentialsAnnotation] == requested.Format(time.RFC3339) {
		requested = requested.Add(time.Second)
	}
	token := requested.Format(time.RFC3339)

	if err := c.annotateRotation(ctx, mg, token); err != nil {
		return "", err
	}
	return token, nil
}

// annotateRotation sets [RotateCredentialsAnnotation] on mg to token.
// It patches the annotation only,
// so it does not conflict with concurrent updates of the controller.
func (c *Client) annotateRotation(ctx context.Context, mg resource.Managed, token string) error {
	before, ok := mg.DeepCopyObject().(resource.Managed)
	if !ok {
		return fmt.Errorf("resource %T does not implement resource.Managed after deepcopy", mg)
	}

	annotations := mg.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[RotateCredentialsAnnotation] = token
	mg.SetAnnotations(annotations)

	if err := c.Patch(ctx, mg, runtimeclient.MergeFrom(before)); err != nil {
		return fmt.Errorf("unable to annotate %v: %w", ObjectName(mg), err)
	}
	return nil
}
