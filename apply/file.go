package apply

import (
	"context"
	"fmt"
	"os"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"golang.org/x/exp/maps"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type fromFile struct{}

func (cmd *Cmd) Run(ctx context.Context, client *api.Client, apply *Cmd) error {
	return File(ctx, cmd.Writer, client, apply.Filename, UpdateOnExists())
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

func File(ctx context.Context, w format.Writer, client *api.Client, file *os.File, opts ...Option) error {
	if file == nil {
		return fmt.Errorf("missing flag -f, --filename=STRING")
	}
	defer file.Close()

	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	obj := &unstructured.Unstructured{}
	if err := yaml.NewYAMLOrJSONDecoder(file, 4096).Decode(obj); err != nil {
		return err
	}

	if cfg.delete {
		if err := client.Delete(ctx, obj); err != nil {
			return err
		}
		w.Successf("🗑", "deleted %s", formatObj(obj))

		return nil
	}

	if err := client.Create(ctx, obj); err != nil {
		if errors.IsAlreadyExists(err) && cfg.updateOnExists {
			if err := update(ctx, client, obj); err != nil {
				return err
			}
			w.Successf("🏗", "applied %s", formatObj(obj))
			return nil
		}
		return err
	}

	w.Successf("🏗", "created %s", formatObj(obj))
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

	return nil
}

func formatObj(obj client.Object) string {
	return fmt.Sprintf("%s %s/%s", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), obj.GetNamespace())
}
