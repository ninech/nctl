package apiresource_test

import (
	"testing"

	"github.com/alecthomas/kong"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/apiresource"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestOfCommand(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	var grammar struct {
		Clusters struct{} `cmd:"" api-resource:"kubernetesclusters"`
		Postgres struct{} `cmd:""`
	}

	parser, err := kong.New(&grammar)
	is.NoError(err)

	resources := map[string]string{}
	for _, node := range parser.Model.Children {
		resources[node.Name] = apiresource.OfCommand(node)
	}

	is.Equal(map[string]string{
		"clusters": "kubernetesclusters",
		"postgres": "postgres",
	}, resources)

	is.Empty(apiresource.OfCommand(nil))
}

func TestFindKind(t *testing.T) {
	t.Parallel()

	scheme, err := api.NewScheme()
	require.NoError(t, err)

	tests := []struct {
		name     string
		resource string
		want     string
	}{
		{name: "plural", resource: "applications", want: "Application"},
		{name: "singular", resource: "application", want: "Application"},
		{name: "name which is both", resource: "postgres", want: "Postgres"},
		{name: "hyphenated as a command names it", resource: "project-config", want: "ProjectConfig"},
		{name: "declared by a command", resource: "kubernetesclusters", want: "KubernetesCluster"},
		{name: "resource of no Nine API", resource: "secrets", want: ""},
		{name: "unknown", resource: "nosuchthing", want: ""},
		{name: "none", resource: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			gvk, err := apiresource.FindKind(scheme, tt.resource)
			if tt.want == "" {
				is.Error(err)
				is.Empty(gvk.Kind)
				return
			}

			is.NoError(err)
			is.Equal(tt.want, gvk.Kind)

			list, err := apiresource.FindListKind(scheme, tt.resource)
			is.NoError(err)
			is.Equal(tt.want+"List", list.Kind)
			is.Equal(gvk.GroupVersion(), list.GroupVersion())
		})
	}
}

func TestFindKindMultipleVersions(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	scheme, err := api.NewScheme()
	is.NoError(err)

	// Register a second version of an already known kind and prefer it over the existing one.
	v1beta1 := schema.GroupVersion{Group: storage.Group, Version: "v1beta1"}
	scheme.AddKnownTypeWithName(v1beta1.WithKind("Postgres"), &storage.Postgres{})
	scheme.AddKnownTypeWithName(v1beta1.WithKind("PostgresList"), &storage.PostgresList{})
	is.NoError(scheme.SetVersionPriority(v1beta1, storage.SchemeGroupVersion))

	// AllKnownTypes is a map, so a single lookup can pick the right kind by chance.
	for range 20 {
		gvk, err := apiresource.FindKind(scheme, "postgres")
		is.NoError(err)
		is.Equal(v1beta1.WithKind("Postgres"), gvk)

		list, err := apiresource.FindListKind(scheme, "postgres")
		is.NoError(err)
		is.Equal(v1beta1.WithKind("PostgresList"), list)
	}
}
