package auth

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/format"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	extensionKind    = "Config"
	extensionGroup   = "nctl.nine.ch"
	extensionVersion = "v1alpha1"
)

var (
	// ErrConfigNotFound describes a missing nctl config in the kubeconfig
	ErrConfigNotFound configError = "nctl config not found"
)

type configError string

func (c configError) Error() string {
	return string(c)
}

// IsConfigNotFoundError returns true if the nctl config could not be found in
// the kubconfig context
func IsConfigNotFoundError(err error) bool {
	return err == ErrConfigNotFound
}

// ReloginNeeded returns an error which outputs the given err with a message
// saying that a re-login is needed.
func ReloginNeeded(err error) error {
	return fmt.Errorf(
		"%w, please re-login by executing %q",
		err,
		format.Command().Login(),
	)
}

// Config is used to store information in the kubeconfig context created
type Config struct {
	metav1.TypeMeta `json:",inline"`
	Organization    string `json:"organization"`
}

func groupVersion() string {
	return fmt.Sprintf("%s/%s", extensionGroup, extensionVersion)
}

func NewConfig(organization string) *Config {
	return &Config{
		TypeMeta: metav1.TypeMeta{
			Kind:       extensionKind,
			APIVersion: groupVersion(),
		},
		Organization: organization,
	}
}

// ToObject wraps a Config in a runtime.Unknown object which implements
// runtime.Object.
func (e *Config) ToObject() (runtime.Object, error) {
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

func parseConfig(o runtime.Object) (*Config, error) {
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

	e := &Config{}
	return e, json.Unmarshal(u.Raw, e)
}

func mergeKubeConfig(from, to *clientcmdapi.Config) {
	for k, v := range from.Clusters {
		to.Clusters[k] = v
	}

	for k, v := range from.AuthInfos {
		to.AuthInfos[k] = v
	}

	for k, v := range from.Contexts {
		to.Contexts[k] = v
	}
}

func RemoveClusterFromKubeConfig(client *api.Client, clusterContext string) error {
	kubeconfig, err := clientcmd.LoadFromFile(client.KubeconfigPath)
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

	return clientcmd.WriteToFile(*kubeconfig, client.KubeconfigPath)
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
	extension, exists := context.Extensions[util.NctlName]
	if !exists {
		return ErrConfigNotFound
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
	context.Extensions[util.NctlName] = cfgObject

	// change project to default for the the given organization:
	context.Namespace = organization

	return clientcmd.WriteToFile(*kubeconfig, kubeconfigPath)
}

func readConfig(kubeconfigContent []byte, contextName string) (*Config, error) {
	kubeconfig, err := clientcmd.Load(kubeconfigContent)
	if err != nil {
		return nil, fmt.Errorf("kubeconfig not found: %w", err)
	}
	context, exists := kubeconfig.Contexts[contextName]
	if !exists {
		return nil, fmt.Errorf("could not find context %q in kubeconfig", contextName)
	}
	extension, exists := context.Extensions[util.NctlName]
	if !exists {
		return nil, ErrConfigNotFound
	}
	cfg, err := parseConfig(extension)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

func ReadConfig(kubeconfigPath string, contextName string) (*Config, error) {
	content, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return nil, err
	}
	return readConfig(content, contextName)
}
