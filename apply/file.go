package apply

import (
	"context"
	"fmt"
	"os"

	"github.com/ninech/nctl/api"
	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type fromFile struct {
}

func (cmd *Cmd) Run(ctx context.Context, client *api.Client, apply *Cmd) error {
	return File(ctx, client, apply.Filename, UpdateOnExists())
}

type Option func(*config)

type config struct {
	updateOnExists bool
	delete         bool
}

func UpdateOnExists() Option {
	return func(c *config) {
		c.updateOnExists = true
	}
}

func Delete() Option {
	return func(c *config) {
		c.delete = true
	}
}

func File(ctx context.Context, client *api.Client, filename string, opts ...Option) error {
	if len(filename) == 0 {
		return fmt.Errorf("missing flag -f, --filename=STRING")
	}

	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	f, err := os.Open(filename)
	if err != nil {
		return err
	}

	obj := &unstructured.Unstructured{}
	if err := yaml.NewYAMLOrJSONDecoder(f, 4096).Decode(obj); err != nil {
		return err
	}

	if cfg.delete {
		if err := client.Delete(ctx, obj); err != nil {
			return err
		}
		printSuccessMessage("deleted", obj)

		return nil
	}

	if err := client.Create(ctx, obj); err != nil {
		if errors.IsAlreadyExists(err) && cfg.updateOnExists {
			return update(ctx, client, obj)
		}
		return err
	}

	printSuccessMessage("created", obj)
	return nil
}

func update(ctx context.Context, client *api.Client, obj *unstructured.Unstructured) error {
	oldObj := &unstructured.Unstructured{}
	oldObj.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())
	if err := client.Get(ctx, api.ObjectName(obj), oldObj); err != nil {
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

	if err := client.Update(ctx, obj); err != nil {
		return err
	}

	printSuccessMessage("applied", obj)
	return nil
}

func printSuccessMessage(message string, obj client.Object) {
	fmt.Printf(
		" ✓ %s %s %s/%s\n", message,
		obj.GetObjectKind().GroupVersionKind().Kind,
		obj.GetName(), obj.GetNamespace(),
	)
}
