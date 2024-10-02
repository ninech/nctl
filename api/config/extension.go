package config

import (
	"encoding/json"
	"fmt"
	"os"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	extensionKind        = "Config"
	extensionGroup       = "nctl.nine.ch"
	extensionVersion     = "v1alpha1"
	NctlExtensionContext = "nctl"
)

var (
	// ErrExtensionNotFound describes a missing extension in the kubeconfig
	ErrExtensionNotFound extensionError = "nctl config not found"
)

type extensionError string

func (c extensionError) Error() string {
	return string(c)
}

// IsExtensionNotFoundError returns true if the nctl config could not be found in
// the kubconfig context
func IsExtensionNotFoundError(err error) bool {
	return err == ErrExtensionNotFound
}

// Extension is used to store custom information in the kubeconfig context
// created
type Extension struct {
	metav1.TypeMeta `json:",inline"`
	Organization    string `json:"organization"`
}

func groupVersion() string {
	return fmt.Sprintf("%s/%s", extensionGroup, extensionVersion)
}

func NewExtension(organization string) *Extension {
	return &Extension{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionKind,
			APIVersion: groupVersion(),
		},
		Organization: organization,
	}
}

// ToObject wraps a Config in a runtime.Unknown object which implements
// runtime.Object.
func (e *Extension) ToObject() (runtime.Object, error) {
	raw, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}
	u := runtime.Unknown{
		Raw:         raw,
		ContentType: runtime.ContentTypeJSON,
	}

	return &u, nil
}

func parseConfig(o runtime.Object) (*Extension, error) {
	u, is := o.(*runtime.Unknown)
	if !is {
		return nil, fmt.Errorf("can not handle underlying type %T", o)
	}

	m := &metav1.TypeMeta{}
	if err := json.Unmarshal(u.Raw, m); err != nil {
		return nil, fmt.Errorf("can not parse type meta of extension: %w", err)
	}
	if m.Kind != extensionKind || m.APIVersion != groupVersion() {
		return nil, fmt.Errorf("can not parse extension with type meta %q", u.TypeMeta.String())
	}

	e := &Extension{}
	return e, json.Unmarshal(u.Raw, e)
}

func readExtension(kubeconfigContent []byte, contextName string) (*Extension, error) {
	kubeconfig, err := clientcmd.Load(kubeconfigContent)
	if err != nil {
		return nil, fmt.Errorf("kubeconfig not found: %w", err)
	}
	context, exists := kubeconfig.Contexts[contextName]
	if !exists {
		return nil, fmt.Errorf("could not find context %q in kubeconfig", contextName)
	}
	extension, exists := context.Extensions[NctlExtensionContext]
	if !exists {
		return nil, ErrExtensionNotFound
	}
	cfg, err := parseConfig(extension)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func ReadExtension(kubeconfigPath string, contextName string) (*Extension, error) {
	content, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return nil, err
	}
	return readExtension(content, contextName)
}

// SetContextOrganization sets the given organization in the given context of the kubeconfig
func SetContextOrganization(kubeconfigPath string, contextName string, organization string) error {
	kubeconfig, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("kubeconfig not found: %w", err)
	}
	context, exists := kubeconfig.Contexts[contextName]
	if !exists {
		return fmt.Errorf("could not find context %q in kubeconfig", contextName)
	}
	extension, exists := context.Extensions[NctlExtensionContext]
	if !exists {
		return ErrExtensionNotFound
	}

	cfg, err := parseConfig(extension)
	if err != nil {
		return err
	}

	if cfg.Organization == organization {
		return nil
	}

	cfg.Organization = organization
	cfgObject, err := cfg.ToObject()
	if err != nil {
		return err
	}
	context.Extensions[NctlExtensionContext] = cfgObject

	// change project to default for the the given organization:
	context.Namespace = organization

	return clientcmd.WriteToFile(*kubeconfig, kubeconfigPath)
}

// SetContextProject sets the given project in the given context of the kubeconfig
func SetContextProject(kubeconfigPath string, contextName string, project string) error {
	kubeconfig, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("kubeconfig not found: %w", err)
	}
	context, exists := kubeconfig.Contexts[contextName]
	if !exists {
		return fmt.Errorf("could not find context %q in kubeconfig", contextName)
	}
	context.Namespace = project
	return clientcmd.WriteToFile(*kubeconfig, kubeconfigPath)
}

// RemoveClusterFromKubeConfig removes the given context from the kubeconfig
func RemoveClusterFromKubeConfig(kubeconfigPath, clusterContext string) error {
	kubeconfig, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("kubeconfig not found: %w", err)
	}

	if _, ok := kubeconfig.Clusters[clusterContext]; !ok {
		return fmt.Errorf("could not find cluster %q in kubeconfig", clusterContext)
	}

	delete(kubeconfig.Clusters, clusterContext)
	delete(kubeconfig.AuthInfos, clusterContext)
	delete(kubeconfig.Contexts, clusterContext)

	kubeconfig.CurrentContext = ""

	return clientcmd.WriteToFile(*kubeconfig, kubeconfigPath)
}

// ContextName returns the kubeconfig context name for the given cluster
func ContextName(cluster *infrastructure.KubernetesCluster) string {
	return fmt.Sprintf("%s/%s", cluster.Name, cluster.Namespace)
}
