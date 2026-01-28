// Package predictor provides shell completion predictors for nctl resources.
package predictor

import (
	"context"
	"os"
	"reflect"
	"strings"
	"time"

	"github.com/gobuffalo/flect"
	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/posener/complete"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	listSuffix  = "list"
	groupSuffix = "nine.ch"
)

// argResourceMap maps certain unusual args to resource names to aid with
// completion.
var argResourceMap = map[string]string{
	"clusters": "kubernetesclusters",
}

// Resource is a predictor that completes resource names by querying the API.
type Resource struct {
	client   *api.Client
	knownGVK *schema.GroupVersionKind
}

// NewResourceName returns a predictor that infers the resource kind from the
// command arguments.
func NewResourceName(client *api.Client) complete.Predictor {
	return &Resource{client: client}
}

// NewResourceNameWithKind returns a predictor for a specific resource kind.
func NewResourceNameWithKind(client *api.Client, gvk schema.GroupVersionKind) complete.Predictor {
	return &Resource{
		client:   client,
		knownGVK: ptr.To(gvk),
	}
}

// Predict returns a list of resource names for shell completion.
func (r *Resource) Predict(args complete.Args) []string {
	u := &unstructured.UnstructuredList{}
	if r.knownGVK != nil {
		u.SetGroupVersionKind(*r.knownGVK)
	} else {
		u.SetGroupVersionKind(r.findKind(args.LastCompleted))
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	ns := r.client.Project
	// if we're looking for projects, we need to use the org as the namespace
	if u.GetObjectKind().GroupVersionKind().Kind == reflect.TypeFor[management.ProjectList]().Name() {
		org, err := r.client.Organization()
		if err != nil {
			return []string{}
		}
		ns = org
	} else {
		// if there is a project set in the args use this
		p, incomplete := findProject(args)
		if incomplete {
			// user is still typing the project flag, don't complete resources
			return []string{}
		}
		if p != "" {
			ns = p
		}
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

// findKind looks up the GroupVersionKind for a given resource argument.
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

// listKindToResource converts a list kind name to its resource name.
func listKindToResource(kind string) string {
	return flect.Pluralize(strings.TrimSuffix(strings.ToLower(kind), listSuffix))
}

// findProject extracts the project from the completion args. It returns the
// project name and a boolean indicating if the project flag is incomplete
// (user is still typing it).
func findProject(args complete.Args) (string, bool) {
	// if the last completed argument is -p or --project, the user is still
	// specifying the project, so we shouldn't complete resources yet
	if args.LastCompleted == "-p" || args.LastCompleted == "--project" {
		return "", true
	}
	if p := findProjectInSlice(args.All); p != "" {
		return p, false
	}
	return "", false
}

// findProjectInSlice searches for -p or --project flag and returns its value.
func findProjectInSlice(args []string) string {
	for i, arg := range args {
		if (arg == "-p" || arg == "--project") && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// NewClient creates an API client configured for shell completion. It uses a
// static token since dynamic exec config breaks with some shells during
// completion.
func NewClient(ctx context.Context, defaultAPICluster string) (*api.Client, error) {
	apiCluster := defaultAPICluster
	if v, ok := os.LookupEnv("NCTL_API_CLUSTER"); ok {
		apiCluster = v
	}
	c, err := api.New(ctx, apiCluster, "", api.StaticToken(ctx))
	if err != nil {
		return nil, err
	}

	return c, nil
}
