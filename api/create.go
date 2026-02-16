package api

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Create saves the object obj in the Kubernetes cluster.
// obj must be a struct pointer so that obj can be updated with the content returned by the Server.
func (c *Client) Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error {
	if c.defaultAnnotations != nil {
		obj.SetAnnotations(c.annotations(obj))
	}

	return c.WithWatch.Create(ctx, obj, opts...)
}
