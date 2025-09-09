package test

import (
	"fmt"
	"os"
	"testing"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/config"
	"github.com/ninech/nctl/api/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

const (
	DefaultProject = "default"
	FakeJWTToken   = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE2ODk2ODkwMDMsImV4cCI6NTE5MjQzMTUwMCwiYXVkIjoid3d3LmV4YW1wbGUuY29tIiwic3ViIjoianJvY2tldEBleGFtcGxlLmNvbSIsImVtYWlsIjoianJvY2tldEBleGFtcGxlLmNvbSIsImdyb3VwcyI6WyIvQ3VzdG9tZXJzL3Rlc3QiLCIvQ3VzdG9tZXJzL2JsYSJdfQ.N6pD8DsPhTK5_Eoy83UNiPNMJ5lbvULdEouDSLE3yak"
	ASAJWTToken    = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJPbmxpbmUgSldUIEJ1aWxkZXIiLCJpYXQiOjE3NTczMzYxNTQsImV4cCI6MTc4ODg3MjE1NCwiYXVkIjoiYXV0aC1zdGFnaW5nLm5pbmUuY2giLCJzdWIiOiJkNGZmOTI5ZS04NGIxLTQ4YWQtOGJkMi0zNWMzMzU0ZWJmMjYiLCJlbWFpbCI6ImV4YW1wbGVAMWI2ZThmOS5zZXJ2aWNlYWNjb3VudC5zdGFnaW5nLm5pbmVhcGlzLmNoIiwib3JnYW5pemF0aW9uIjoibmluZXRlc3QiLCJwcm9qZWN0IjoibmluZXRlc3QtZm9vIn0.9sk9nw3miwn58kXhStRfwfVJZxlcABzod-W5Yt7LNog"
)

type clientSetup struct {
	organization     string
	defaultProject   string
	projects         []string
	objects          []client.Object
	kubeconfig       *kubeconfigSetup
	nameIndexesOn    []runtime.Object
	interceptorFuncs *interceptor.Funcs
}

type kubeconfigSetup struct {
	t *testing.T
}

type ClientSetupOption func(*clientSetup)

func defaultClientSetup() *clientSetup {
	return &clientSetup{
		organization:   DefaultProject,
		defaultProject: DefaultProject,
		objects:        []client.Object{},
	}
}

// WithOrganization sets the organization of the client
func WithOrganization(org string) ClientSetupOption {
	return func(cs *clientSetup) {
		cs.organization = org
	}
}

// WithProjects exclusively sets the projects which should be created
func WithProjects(projects ...string) ClientSetupOption {
	return func(cs *clientSetup) {
		cs.projects = projects
	}
}

// WithProjectsFromResources reads the namespaces of all given resources and
// adds them (unique) to the clientSetups project list.
// Use this on your own risk in combination with `WithProjects`. Make make sure that
// `WithProjects` is set before this function call in a functional options
// list. Otherwise you will overwrite the results from this function.
func WithProjectsFromResources(resources ...client.Object) ClientSetupOption {
	return func(cs *clientSetup) {
		seen := make(map[string]struct{})

	OUTER:
		for _, v := range resources {
			if _, ok := seen[v.GetNamespace()]; !ok {
				seen[v.GetNamespace()] = struct{}{}
			} else {
				continue
			}
			for _, p := range cs.projects {
				if p == v.GetNamespace() {
					continue OUTER
				}
			}
			cs.projects = append(cs.projects, v.GetNamespace())
		}
	}
}

// WithDefaultProject sets the default project of the client
func WithDefaultProject(project string) ClientSetupOption {
	return func(cs *clientSetup) {
		cs.defaultProject = project
	}
}

// WithObjects sets the objects which can be returned from the client
func WithObjects(objects ...client.Object) ClientSetupOption {
	return func(cs *clientSetup) {
		cs.objects = objects
	}
}

// WithKubeconfig creates a fake kubeconfig which gets removed once the passed
// test finished
func WithKubeconfig(t *testing.T) ClientSetupOption {
	return func(cs *clientSetup) {
		cs.kubeconfig = &kubeconfigSetup{t: t}
	}
}

// WithNameIndexFor adds a name index to the fake client set for the passed object type
func WithNameIndexFor(obj runtime.Object) ClientSetupOption {
	return func(cs *clientSetup) {
		cs.nameIndexesOn = append(cs.nameIndexesOn, obj)
	}
}

func WithInterceptorFuncs(f interceptor.Funcs) ClientSetupOption {
	return func(cs *clientSetup) {
		cs.interceptorFuncs = &f
	}
}

func SetupClient(opts ...ClientSetupOption) (*api.Client, error) {
	setup := defaultClientSetup()
	for _, opt := range opts {
		opt(setup)
	}

	scheme, err := api.NewScheme()
	if err != nil {
		return nil, err
	}
	resources := []client.Object{namespace(setup.organization)}
	resources = append(resources, Projects(setup.organization, setup.projects...)...)
	for _, proj := range setup.projects {
		// do not create the namespace for the organisation project
		// again
		if proj == setup.organization {
			continue
		}
		resources = append(resources, namespace(proj))
	}
	resources = append(resources, setup.objects...)

	clientBuilder := fake.NewClientBuilder().WithScheme(scheme).WithObjects(resources...)
	for _, res := range setup.nameIndexesOn {
		clientBuilder = clientBuilder.WithIndex(res, "metadata.name", func(o client.Object) []string {
			return []string{o.GetName()}
		})
	}
	if setup.interceptorFuncs != nil {
		clientBuilder = clientBuilder.WithInterceptorFuncs(*setup.interceptorFuncs)
	}
	client := clientBuilder.Build()

	c := &api.Client{
		Config:    &rest.Config{BearerToken: FakeJWTToken},
		WithWatch: client, Project: setup.defaultProject,
	}

	if setup.kubeconfig == nil {
		return c, nil
	}
	fName, err := CreateTestKubeconfig(c, setup.organization)
	if err != nil {
		return nil, fmt.Errorf("error on kubeconfig creation: %w", err)
	}
	if setup.kubeconfig.t == nil {
		return c, nil
	}
	t := setup.kubeconfig.t
	t.Cleanup(func() {
		if err := os.Remove(fName); err != nil {
			t.Errorf("can not remove kubeconfig file: %v", err)
		}
	})
	return c, nil
}

func namespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// CreateTestKubeconfig creates a test kubeconfig which contains a nctl
// extension config with the given organization
func CreateTestKubeconfig(client *api.Client, organization string) (string, error) {
	var extensions map[string]runtime.Object
	if organization != "" {
		cfg := config.NewExtension(organization)
		cfgObject, err := cfg.ToObject()
		if err != nil {
			return "", err
		}
		extensions = map[string]runtime.Object{
			util.NctlName: cfgObject,
		}
	}

	contextName := "test"
	kubeconfig := clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			contextName: {
				Server: "not.so.important",
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			contextName: {
				Token: "not-valid",
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			contextName: {
				Cluster:    contextName,
				AuthInfo:   contextName,
				Namespace:  "default",
				Extensions: extensions,
			},
		},
		CurrentContext: contextName,
	}

	// create and open a temporary file
	f, err := os.CreateTemp("", "kubeconfig-")
	if err != nil {
		return "", err
	}
	defer f.Close()

	content, err := clientcmd.Write(kubeconfig)
	if err != nil {
		return "", err
	}
	if _, err = f.Write(content); err != nil {
		return "", err
	}
	client.KubeconfigContext = contextName
	client.KubeconfigPath = f.Name()

	return f.Name(), nil
}
