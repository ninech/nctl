// Package serviceconnection holds what the commands managing service connections share.
package serviceconnection

import (
	"github.com/alecthomas/kong"
	networking "github.com/ninech/apis/networking/v1alpha1"

	"github.com/ninech/nctl/internal/flag"
)

// KubernetesClusterOptions declares the flags which configure a KubernetesCluster as the source of a service connection.
// Embed it with the prefix the flags are to be registered under,
// the create and update commands both use "source-".
// https://pkg.go.dev/github.com/ninech/apis@v0.0.0-20250708054129-4d49f7a6c606/networking/v1alpha1#KubernetesClusterOptions
type KubernetesClusterOptions struct {
	PodSelector       *flag.LabelSelector `placeholder:"${label_selector_placeholder}" help:"${label_selector_requirements} Restrict which pods of the KubernetesCluster can connect to the service connection destination. If left empty, all pods are allowed. If the namespace selector is also set, then the pod selector as a whole selects the pods matching pod selector in the namespaces selected by namespace selector.\n\n${label_selector_usage}."`
	NamespaceSelector *flag.LabelSelector `placeholder:"${label_selector_placeholder}" help:"${label_selector_requirements} Select namespaces using labels set on namespaces. If left empty, all namespaces are selected. Allows to further restrict the pods selected by the PodSelector.\n\n${label_selector_usage}."`
}

// APIType returns the API type [networking.KubernetesClusterOptions] of the [KubernetesClusterOptions],
// or nil if none of the selectors was passed.
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

// KongVars returns the variables which the help of the [KubernetesClusterOptions] flags interpolates.
func KongVars() kong.Vars {
	result := make(kong.Vars)
	result["label_selector_placeholder"] = "'key1=value1,key2=value2,key3 in (value3)'"
	result["label_selector_usage"] = "Selector (label query) to filter on, supports '=', '==', '!=', 'in', 'notin'. Matching objects must satisfy all of the specified label constraints."
	result["label_selector_requirements"] = "Can only be set when the source is a KubernetesCluster."

	return result
}
