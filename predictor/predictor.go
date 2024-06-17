package predictor

import (
	"context"
	"reflect"
	"strings"
	"time"

	"github.com/gobuffalo/flect"
	"github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/auth"
	"github.com/posener/complete"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	listSuffix  = "list"
	groupSuffix = "nine.ch"
)

// argResourceMap maps certain unusual args to resource names to aid with
// completion.
var argResourceMap = map[string]string{
	"clusters":    "kubernetesclusters",
	"set-project": "projects",
	"-p":          "projects",
	"--project":   "projects",
}

type Resource struct {
	client *api.Client
}

func NewResourceName(clientCreator func() (*api.Client, error)) *Resource {
	c, err := clientCreator()
	if err != nil {
		return &Resource{}
	}

	return &Resource{client: c}
}

func (r *Resource) Predict(args complete.Args) []string {
	if r.client == nil {
		return []string{}
	}

	u := &unstructured.UnstructuredList{}
	u.SetGroupVersionKind(r.findKind(args.LastCompleted))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	ns := r.client.Project
	// if we're looking for projects, we need to use the org as the namespace
	if u.GetObjectKind().GroupVersionKind().Kind == reflect.TypeOf(v1alpha1.ProjectList{}).Name() {
		cfg, err := auth.ReadConfig(r.client.KubeconfigPath, r.client.KubeconfigContext)
		if err != nil {
			return []string{}
		}
		ns = cfg.Organization
	}

	if err := r.client.List(ctx, u, client.InNamespace(ns)); err != nil {
		return []string{}
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

	for gvk := range r.client.Scheme().AllKnownTypes() {
		if !strings.HasSuffix(strings.ToLower(gvk.Kind), listSuffix) {
			continue
		}
		if strings.HasSuffix(strings.ToLower(gvk.Group), groupSuffix) &&
			listKindToResource(gvk.Kind) == flect.Pluralize(arg) {
			return gvk
		}
	}

	return schema.GroupVersionKind{}
}

func listKindToResource(kind string) string {
	return flect.Pluralize(strings.TrimSuffix(strings.ToLower(kind), listSuffix))
}
