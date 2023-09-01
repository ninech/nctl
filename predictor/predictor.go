package predictor

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/gobuffalo/flect"
	"github.com/ninech/nctl/api"
	"github.com/posener/complete"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Resource struct {
	client *api.Client
}

var argResourceMap = map[string]string{
	"set-project": "projects",
}

func NewResource(addr func() string) *Resource {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	c, err := api.New(ctx, addr(), "")
	if err != nil {
		return &Resource{}
	}

	return &Resource{client: c}
}

func (r *Resource) Predict(args complete.Args) []string {
	if r.client == nil {
		return []string{}
	}

	kind := r.findKind(args.LastCompleted)

	u := &unstructured.UnstructuredList{}
	u.SetGroupVersionKind(kind)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	if err := r.client.List(ctx, u, client.InNamespace(r.client.Project)); err != nil {
		log.Println(err)
	}

	resources := make([]string, len(u.Items))
	for _, res := range u.Items {
		resources = append(resources, res.GetName())
	}

	return resources
}

func (r *Resource) findKind(arg string) schema.GroupVersionKind {
	if v, ok := argResourceMap[arg]; ok {
		arg = v
	}

	resource := flect.Pluralize(arg)

	for gvk := range r.client.Scheme().AllKnownTypes() {
		if !strings.HasSuffix(strings.ToLower(gvk.Kind), "list") {
			continue
		}
		if strings.HasSuffix(strings.ToLower(gvk.Group), "nine.ch") &&
			listKindToResource(gvk.Kind) == resource {
			return gvk
		}
	}

	return schema.GroupVersionKind{}
}

func listKindToResource(kind string) string {
	return flect.Pluralize(strings.TrimSuffix(strings.ToLower(kind), "list"))
}
