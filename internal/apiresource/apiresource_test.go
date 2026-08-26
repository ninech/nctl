package apiresource_test

import (
	"testing"

	"github.com/alecthomas/kong"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/apiresource"
	"github.com/stretchr/testify/require"
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
