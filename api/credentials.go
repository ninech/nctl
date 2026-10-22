package api

import (
	"context"
	"fmt"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"k8s.io/apimachinery/pkg/types"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// RotateCredentials makes the controller of mg generate new credentials
// and write them to a new connection secret.
// It deletes the connection secret of mg and then sets
// [RotateCredentialsAnnotation] on mg to the current time,
// like kubectl rollout restart does.
//
// It returns the UID of the deleted secret,
// which is empty if the secret was missing already.
func (c *Client) RotateCredentials(ctx context.Context, mg resource.Managed) (types.UID, error) {
	secret, err := c.GetConnectionSecret(ctx, mg)
	// A missing secret is being regenerated already,
	// the annotation makes sure it happens right away.
	if runtimeclient.IgnoreNotFound(err) != nil {
		return "", err
	}

	var old types.UID
	if err == nil {
		old = secret.UID
		if err := c.Delete(ctx, secret, runtimeclient.Preconditions{UID: &old}); runtimeclient.IgnoreNotFound(err) != nil {
			return "", fmt.Errorf("unable to delete connection secret %v: %w", ObjectName(secret), err)
		}
	}

	// The annotation has to be set after the deletion,
	// as the reconcile would find the old secret otherwise.
	if err := c.annotateRotation(ctx, mg); err != nil {
		return "", err
	}
	return old, nil
}

// annotateRotation sets [RotateCredentialsAnnotation] on mg.
// It patches the annotation only,
// so it does not conflict with concurrent updates of the controller.
func (c *Client) annotateRotation(ctx context.Context, mg resource.Managed) error {
	before, ok := mg.DeepCopyObject().(resource.Managed)
	if !ok {
		return fmt.Errorf("resource %T does not implement resource.Managed after deepcopy", mg)
	}

	annotations := mg.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[RotateCredentialsAnnotation] = time.Now().UTC().Format(time.RFC3339)
	mg.SetAnnotations(annotations)

	if err := c.Patch(ctx, mg, runtimeclient.MergeFrom(before)); err != nil {
		return fmt.Errorf("unable to annotate %v: %w", ObjectName(mg), err)
	}
	return nil
}
