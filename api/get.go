package api

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// GetObject gets the object in the current client project with some
// ux-improvements like hinting when the object has been found in a different
// project of the same organization.
func (c *Client) GetObject(ctx context.Context, name string, obj runtimeclient.Object) error {
	list := &unstructured.UnstructuredList{}
	gvks, _, err := c.Scheme().ObjectKinds(obj)
	if err != nil || len(gvks) != 1 {
		return fmt.Errorf("unable to determine GVK from object %T", obj)
	}
	list.SetGroupVersionKind(gvks[0])
	if err := c.ListObjects(ctx, list, MatchName(name)); err != nil {
		return err
	}
	// this *should* already be handled by ListN
	if len(list.Items) == 0 {
		return fmt.Errorf("resource %q was not found in any project", name)
	}
	return runtime.DefaultUnstructuredConverter.FromUnstructured(list.Items[0].Object, obj)
}
