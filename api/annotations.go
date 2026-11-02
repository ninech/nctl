package api

import (
	"context"

	"github.com/ninech/nctl/internal/cli"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ManagedByAnnotation is the annotation key marking objects that nctl created or updated.
	ManagedByAnnotation = "app.kubernetes.io/managed-by"
	// Name is the name of the CLI and the value of ManagedByAnnotation on objects managed by it.
	Name = cli.Name
	// RotateCredentialsAnnotation is the annotation key nctl sets to the time
	// it reset the credentials of a resource,
	// which makes the controller reconcile the resource.
	RotateCredentialsAnnotation = "nine.ch/rotate-credentials"
	// CreatedByAnnotation is the annotation key holding the account which
	// created a resource. Only resource kinds which opt in carry it.
	CreatedByAnnotation = "nine.ch/created-by"
)

// IsManagedBy reports whether the annotations mark an object as managed by nctl.
func IsManagedBy(annotations map[string]string) bool {
	if annotations == nil {
		return false
	}

	return annotations[ManagedByAnnotation] == Name
}

// SetCreatedBy sets [CreatedByAnnotation] on obj to the account of the
// current user. It is not a default annotation, because updates must not
// change the creator. An existing value is kept. Nothing is set if the
// account is unknown, as the creator is informational only.
func (c *Client) SetCreatedBy(ctx context.Context, obj runtimeclient.Object) {
	annotations := obj.GetAnnotations()
	if _, ok := annotations[CreatedByAnnotation]; ok {
		return
	}

	userInfo, err := GetUserInfoFromToken(c.Token(ctx))
	if err != nil || userInfo.User == "" {
		return
	}

	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[CreatedByAnnotation] = userInfo.User
	obj.SetAnnotations(annotations)
}
