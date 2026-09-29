package api

import (
	"context"
	"errors"
	"io"
	"maps"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// ApplyResult tells what [Client.ApplyManifest] did with the object of the
// manifest.
type ApplyResult string

const (
	ApplyResultCreated ApplyResult = "created"
	ApplyResultUpdated ApplyResult = "updated"
)

// CreateManifest creates the object described by the YAML or JSON manifest
// read from r and returns it.
func (c *Client) CreateManifest(ctx context.Context, r io.Reader) (*unstructured.Unstructured, error) {
	obj, err := decodeManifest(r)
	if err != nil {
		return nil, err
	}
	if err := c.Create(ctx, obj); err != nil {
		return nil, err
	}

	return obj, nil
}

// ApplyManifest creates the object described by the YAML or JSON manifest
// read from r, or updates it if it already exists, and returns it along with
// what happened.
func (c *Client) ApplyManifest(ctx context.Context, r io.Reader) (*unstructured.Unstructured, ApplyResult, error) {
	obj, err := decodeManifest(r)
	if err != nil {
		return nil, "", err
	}
	if err := c.Create(ctx, obj); err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return nil, "", err
		}
		if err := c.updateExisting(ctx, obj); err != nil {
			return nil, "", err
		}

		return obj, ApplyResultUpdated, nil
	}

	return obj, ApplyResultCreated, nil
}

// DeleteManifest deletes the object described by the YAML or JSON manifest
// read from r and returns it.
func (c *Client) DeleteManifest(ctx context.Context, r io.Reader) (*unstructured.Unstructured, error) {
	obj, err := decodeManifest(r)
	if err != nil {
		return nil, err
	}
	if err := c.Delete(ctx, obj); err != nil {
		return nil, err
	}

	return obj, nil
}

// decodeManifest decodes the first YAML or JSON document read from r.
func decodeManifest(r io.Reader) (*unstructured.Unstructured, error) {
	if r == nil {
		return nil, errors.New("no manifest given")
	}

	obj := &unstructured.Unstructured{}
	if err := yaml.NewYAMLOrJSONDecoder(r, 4096).Decode(obj); err != nil {
		return nil, err
	}

	return obj, nil
}

// updateExisting replaces the object in the cluster with obj while keeping
// the annotations, labels and finalizers of the existing object that obj does
// not set itself.
func (c *Client) updateExisting(ctx context.Context, obj *unstructured.Unstructured) error {
	oldObj := &unstructured.Unstructured{}
	oldObj.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())
	if err := c.Get(ctx, ObjectName(obj), oldObj); err != nil {
		return err
	}

	// merge annotations/labels
	obj.SetAnnotations(mergeInto(oldObj.GetAnnotations(), obj.GetAnnotations()))
	obj.SetLabels(mergeInto(oldObj.GetLabels(), obj.GetLabels()))

	// preserve finalizers
	obj.SetFinalizers(append(obj.GetFinalizers(), oldObj.GetFinalizers()...))
	// ensure resource version is up to date
	obj.SetResourceVersion(oldObj.GetResourceVersion())

	return c.Update(ctx, obj)
}

// mergeInto copies src into dst and returns dst. A nil dst is allocated when
// there is something to copy and stays nil otherwise.
func mergeInto(dst, src map[string]string) map[string]string {
	if dst == nil && len(src) > 0 {
		dst = make(map[string]string, len(src))
	}
	maps.Copy(dst, src)

	return dst
}
