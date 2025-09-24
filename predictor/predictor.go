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

type Resource struct {
	client   *api.Client
	knownGVK *schema.GroupVersionKind
}

func NewResourceName(client *api.Client) complete.Predictor {
	return &Resource{client: client}
}

func NewResourceNameWithKind(client *api.Client, gvk schema.GroupVersionKind) complete.Predictor {
	return &Resource{
		client:   client,
		knownGVK: ptr.To(gvk),
	}
}

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
	if u.GetObjectKind().GroupVersionKind().Kind == reflect.TypeOf(management.ProjectList{}).Name() {
		org, err := r.client.Organization()
		if err != nil {
			return []string{}
		}
		ns = org
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

func NewClient(ctx context.Context, defaultAPICluster string) (*api.Client, error) {
	// the client for the predictor requires a static token in the client config
	// since dynamic exec config seems to break with some shells during completion.
	// The exact reason for that is unknown.
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
