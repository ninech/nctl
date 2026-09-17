package api

import (
	"context"
	"fmt"
	"maps"
	"os"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// ApplyResult tells what [Client.ApplyFromFile] did with the object of the
// manifest.
type ApplyResult string

const (
	ApplyResultCreated ApplyResult = "created"
	ApplyResultUpdated ApplyResult = "updated"
)

// CreateFromFile creates the object described by the YAML or JSON manifest in
// file and returns it. file is closed before returning.
func (c *Client) CreateFromFile(ctx context.Context, file *os.File) (*unstructured.Unstructured, error) {
	obj, err := decodeManifest(file)
	if err != nil {
		return nil, err
	}
	if err := c.Create(ctx, obj); err != nil {
		return nil, err
	}

	return obj, nil
}

// ApplyFromFile creates the object described by the YAML or JSON manifest in
// file, or updates it if it already exists, and returns it along with what
// happened. file is closed before returning.
func (c *Client) ApplyFromFile(ctx context.Context, file *os.File) (*unstructured.Unstructured, ApplyResult, error) {
	obj, err := decodeManifest(file)
	if err != nil {
		return nil, "", err
	}
	if err := c.Create(ctx, obj); err != nil {
		if !errors.IsAlreadyExists(err) {
			return nil, "", err
		}
		if err := c.updateExisting(ctx, obj); err != nil {
			return nil, "", err
		}

		return obj, ApplyResultUpdated, nil
	}

	return obj, ApplyResultCreated, nil
}

// DeleteFromFile deletes the object described by the YAML or JSON manifest in
// file and returns it. file is closed before returning.
func (c *Client) DeleteFromFile(ctx context.Context, file *os.File) (*unstructured.Unstructured, error) {
	obj, err := decodeManifest(file)
	if err != nil {
		return nil, err
	}
	if err := c.Delete(ctx, obj); err != nil {
		return nil, err
	}

	return obj, nil
}

// decodeManifest decodes the first YAML or JSON document of file and closes
// it.
func decodeManifest(file *os.File) (*unstructured.Unstructured, error) {
	if file == nil {
		return nil, fmt.Errorf("missing flag -f, --filename=STRING")
	}
	defer file.Close()

	obj := &unstructured.Unstructured{}
	if err := yaml.NewYAMLOrJSONDecoder(file, 4096).Decode(obj); err != nil {
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
	annotations, labels := oldObj.GetAnnotations(), oldObj.GetLabels()
	maps.Copy(annotations, obj.GetAnnotations())
	maps.Copy(labels, obj.GetLabels())
	obj.SetAnnotations(annotations)
	obj.SetLabels(labels)

	// preserve finalizers
	obj.SetFinalizers(append(obj.GetFinalizers(), oldObj.GetFinalizers()...))
	// ensure resource version is up to date
	obj.SetResourceVersion(oldObj.GetResourceVersion())

	return c.Update(ctx, obj)
}
