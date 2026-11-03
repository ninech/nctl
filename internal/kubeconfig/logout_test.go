package kubeconfig

import (
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestRemoveContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		config          *clientcmdapi.Config
		wantClusters    []string
		wantCurrent     string
		wantErrNotExist bool
	}{
		{
			name:        "removes the context, its user and its cluster",
			config:      merged(singleEntryConfig("api"), singleEntryConfig("other")),
			wantCurrent: "",
		},
		{
			name: "keeps the current context if it is another one",
			config: func() *clientcmdapi.Config {
				c := merged(singleEntryConfig("api"), singleEntryConfig("other"))
				c.CurrentContext = "other"
				return c
			}(),
			wantCurrent: "other",
		},
		{
			name: "keeps a cluster another context uses",
			config: func() *clientcmdapi.Config {
				c := merged(singleEntryConfig("api"), singleEntryConfig("other"))
				c.Contexts["other"].Cluster = "api"
				return c
			}(),
			wantClusters: []string{"api"},
		},
		{
			name:            "fails if the context does not exist",
			config:          singleEntryConfig("other"),
			wantErrNotExist: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)
			path := filepath.Join(t.TempDir(), "kubeconfig")
			is.NoError(clientcmd.WriteToFile(*tt.config, path))

			err := RemoveContext(&clientcmd.ClientConfigLoadingRules{ExplicitPath: path}, "api")
			if tt.wantErrNotExist {
				is.ErrorIs(err, fs.ErrNotExist)
				return
			}
			is.NoError(err)

			got, err := clientcmd.LoadFromFile(path)
			is.NoError(err)
			is.NotContains(got.Contexts, "api")
			is.NotContains(got.AuthInfos, "api")
			is.Contains(got.Contexts, "other")
			is.Contains(got.AuthInfos, "other")
			is.ElementsMatch(append([]string{"other"}, tt.wantClusters...), keys(got.Clusters))
			is.Equal(tt.wantCurrent, got.CurrentContext)
		})
	}
}

func TestRemoveContextFromFileDefiningIt(t *testing.T) {
	t.Parallel()
	is := require.New(t)
	dir := t.TempDir()
	apiPath, otherPath := filepath.Join(dir, "api"), filepath.Join(dir, "other")
	is.NoError(clientcmd.WriteToFile(*singleEntryConfig("api"), apiPath))
	is.NoError(clientcmd.WriteToFile(*singleEntryConfig("other"), otherPath))

	is.NoError(RemoveContext(&clientcmd.ClientConfigLoadingRules{Precedence: []string{otherPath, apiPath}}, "api"))

	api, err := clientcmd.LoadFromFile(apiPath)
	is.NoError(err)
	is.Empty(api.Contexts)
	is.Empty(api.AuthInfos)
	is.Empty(api.Clusters)
	other, err := clientcmd.LoadFromFile(otherPath)
	is.NoError(err)
	is.Equal(singleEntryConfig("other").Contexts["other"].Cluster, other.Contexts["other"].Cluster)
	is.Contains(other.AuthInfos, "other")
	is.Equal("other", other.CurrentContext)
}

// merged returns the entries of configs in one config whose current context is the one of the first.
func merged(configs ...*clientcmdapi.Config) *clientcmdapi.Config {
	c := clientcmdapi.NewConfig()
	c.CurrentContext = configs[0].CurrentContext
	for _, from := range configs {
		mergeKubeConfig(from, c)
	}
	return c
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
