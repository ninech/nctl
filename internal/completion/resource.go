package completion

import (
	"context"
	"fmt"
	"reflect"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/internal/apiresource"
	"github.com/posener/complete"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// resourcePredictor completes the names of a single API resource by querying
// the API.
type resourcePredictor struct {
	client  clientFunc
	project projectFinder

	// resource is the name of the API resource to list. It is bound to the
	// command being completed while the completion tree is built, see
	// [bindResourceNames].
	resource string
	// knownGVK lists that kind instead of resolving resource, for
	// predictors registered for a fixed kind rather than for a command.
	knownGVK *schema.GroupVersionKind
}

// newResourceName returns a predictor for the named API resource.
func newResourceName(client clientFunc, project projectFinder, resource string) *resourcePredictor {
	return &resourcePredictor{client: client, project: project, resource: resource}
}

// newResourceNameWithKind returns a predictor for a specific resource kind.
func newResourceNameWithKind(client clientFunc, project projectFinder, gvk schema.GroupVersionKind) *resourcePredictor {
	return &resourcePredictor{
		client:   client,
		project:  project,
		knownGVK: new(gvk),
	}
}

func (r *resourcePredictor) Predict(args complete.Args) []string {
	client, err := r.client()
	if err != nil {
		return fail(err)
	}

	u := &unstructured.UnstructuredList{}
	if r.knownGVK != nil {
		u.SetGroupVersionKind(*r.knownGVK)
	} else {
		gvk, err := apiresource.FindListKind(client.Scheme(), r.resource)
		if err != nil {
			return fail(err)
		}
		u.SetGroupVersionKind(gvk)
	}
	kind := u.GetObjectKind().GroupVersionKind()

	ctx, cancel := context.WithTimeout(context.Background(), apiTimeout)
	defer cancel()

	ns := client.Project
	// Projects are scoped to the organization namespace.
	if kind.Kind == reflect.TypeFor[management.ProjectList]().Name() {
		org, err := client.Organization()
		if err != nil {
			return fail(fmt.Errorf("cannot determine the organization: %w", err))
		}
		ns = org
	} else {
		p, incomplete := r.project.find(args)
		if incomplete {
			return nil
		}
		if p != "" {
			ns = p
		}
	}

	if err := client.List(ctx, u, runtimeclient.InNamespace(ns)); err != nil {
		return fail(fmt.Errorf("cannot list %s in project %q: %w", kind.Kind, ns, err))
	}

	resources := make([]string, 0, len(u.Items))
	for _, res := range u.Items {
		resources = append(resources, res.GetName())
	}

	return resources
}
