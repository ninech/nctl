package create

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/alecthomas/kong"
	meta "github.com/ninech/apis/meta/v1alpha1"
	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/api"
)

type serviceConnectionCmd struct {
	resourceCmd
	Source                   TypedReference           `placeholder:"kind/name" help:"The source of the connection in the form kind/name. Allowed source kinds are: ${allowed_sources}." required:""`
	Destination              TypedReference           `placeholder:"kind/name" help:"The destination of the connection in the form kind/name. Must be in the same project as the service connection. Allowed destination kinds are: ${allowed_destinations}." required:""`
	SourceNamespace          string                   `help:"The source namespace of the connection. Defaults to current project."`
	KubernetesClusterOptions KubernetesClusterOptions `embed:"" prefix:"source-"`
}

// KubernetesClusterOptions
// https://pkg.go.dev/github.com/ninech/apis@v0.0.0-20250708054129-4d49f7a6c606/networking/v1alpha1#KubernetesClusterOptions
type KubernetesClusterOptions struct {
	PodSelector       *LabelSelector `placeholder:"${label_selector_placeholder}" help:"${label_selector_requirements} Can be used to restrict which pods of the KubernetesCluster can connect to the service connection destination. If left empty, all pods are allowed. If the namespace selector is also set, then the pod selector as a whole selects the pods matching pod selector in the namespaces selected by namespace selector.\n\n${label_selector_usage}"`
	NamespaceSelector *LabelSelector `placeholder:"${label_selector_placeholder}" help:"${label_selector_requirements} Selects namespaces using labels set on namespaces. If left empty, it selects all namespaces. It can be used to further restrict the pods selected by the PodSelector.\n\n${label_selector_usage}"`
}

// APIType returns the API type [networking.KubernetesClusterOptions] of the [KubernetesClusterOptions].
func (kco *KubernetesClusterOptions) APIType() *networking.KubernetesClusterOptions {
	if kco == nil || (kco.PodSelector == nil && kco.NamespaceSelector == nil) {
		return nil
	}

	nkco := &networking.KubernetesClusterOptions{}
	if kco.PodSelector != nil {
		nkco.PodSelector.MatchLabels = kco.PodSelector.MatchLabels
		nkco.PodSelector.MatchExpressions = kco.PodSelector.MatchExpressions
	}
	if kco.NamespaceSelector != nil {
		nkco.NamespaceSelector.MatchLabels = kco.NamespaceSelector.MatchLabels
		nkco.NamespaceSelector.MatchExpressions = kco.NamespaceSelector.MatchExpressions
	}

	return nkco
}

// LabelSelector is a label query over a set of resources.
// https://pkg.go.dev/k8s.io/kubectl@v0.33.2/pkg/cmd/util#AddLabelSelectorFlagVar
type LabelSelector struct {
	metav1.LabelSelector
}

// UnmarshalText parses a label selector from a string.
// https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#list-and-watch-filtering
func (ls *LabelSelector) UnmarshalText(text []byte) error {
	s := strings.TrimSpace(string(text))
	if s == "" {
		return nil
	}

	selector, err := metav1.ParseToLabelSelector(s)
	if err != nil {
		return fmt.Errorf("error parsing %q: %w", s, err)
	}
	ls.LabelSelector = *selector

	return nil
}

// TypedReference is a reference to a resource with a specific type.
type TypedReference struct {
	meta.TypedReference
}

// UnmarshalText parses a typed reference from a string.
func (r *TypedReference) UnmarshalText(text []byte) error {
	s := strings.TrimSpace(string(text))
	kind, name, found := strings.Cut(s, "/")
	if !found || kind == "" || name == "" {
		return fmt.Errorf("unmarshal error: expected kind/name, got %q", text)
	}

	groupKind, err := groupKindFromKind(kind)
	if err != nil {
		return fmt.Errorf("unmarshal error: %w", err)
	}

	r.Name = name
	r.GroupKind = groupKind

	return nil
}

func (cmd *serviceConnectionCmd) Run(ctx context.Context, client *api.Client) error {
	sc, err := cmd.newServiceConnection(client.Project)
	if err != nil {
		return err
	}

	c := newCreator(client, sc, networking.ServiceConnectionKind)
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !cmd.Wait {
		return nil
	}

	return c.wait(ctx, waitStage{
		objectList: &networking.ServiceConnectionList{},
		onResult:   resourceAvailable,
	},
	)
}

func (cmd *serviceConnectionCmd) newServiceConnection(namespace string) (*networking.ServiceConnection, error) {
	name := getName(cmd.Name)

	if cmd.SourceNamespace == "" {
		cmd.SourceNamespace = namespace
	}
	cmd.Source.Namespace = cmd.SourceNamespace

	cmd.Destination.Namespace = namespace

	sc := &networking.ServiceConnection{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: networking.ServiceConnectionSpec{
			ForProvider: networking.ServiceConnectionParameters{
				Source: networking.Source{
					Reference: cmd.Source.TypedReference,
				},
				Destination: cmd.Destination.TypedReference,
			},
		},
	}

	sc.Spec.ForProvider.Source.KubernetesClusterOptions = cmd.KubernetesClusterOptions.APIType()

	return sc, nil
}

func groupKindFromKind(kind string) (metav1.GroupKind, error) {
	scheme, err := api.NewScheme()
	if err != nil {
		return metav1.GroupKind{}, fmt.Errorf("error creating scheme: %w", err)
	}

	for gvk := range scheme.AllKnownTypes() {
		if kind == strings.ToLower(gvk.Kind) {
			return metav1.GroupKind(gvk.GroupKind()), nil
		}
	}

	return metav1.GroupKind{}, fmt.Errorf("kind %s is invalid", kind)
}

// ServiceConnectionKongVars returns all variables which are used in the ServiceConnection
// create command
func ServiceConnectionKongVars() kong.Vars {
	result := make(kong.Vars)
	result["allowed_sources"] = "kubernetescluster, application"
	result["allowed_destinations"] = "keyvaluestore"
	result["label_selector_placeholder"] = "'key1=value1,key2=value2,key3 in (value3)'"
	result["label_selector_usage"] = "Selector (label query) to filter on, supports '=', '==', '!=', 'in', 'notin'. Matching objects must satisfy all of the specified label constraints."
	result["label_selector_requirements"] = "Can only be set when the source is a KubernetesCluster."

	return result
}
