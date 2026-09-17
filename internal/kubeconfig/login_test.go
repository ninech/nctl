package kubeconfig

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/ninech/nctl/internal/format"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func singleEntryConfig(name string) *clientcmdapi.Config {
	return &clientcmdapi.Config{
		Clusters:       map[string]*clientcmdapi.Cluster{name: {Server: "https://" + name}},
		AuthInfos:      map[string]*clientcmdapi.AuthInfo{name: {Token: name}},
		Contexts:       map[string]*clientcmdapi.Context{name: {Cluster: name, AuthInfo: name}},
		CurrentContext: name,
	}
}

func TestLogin(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		existing       *clientcmdapi.Config
		userName       string
		toOrg          string
		opts           []LoginOption
		wantContext    string
		wantEntries    int
		wantProject    string
		wantOutput     []string
		unwantedOutput []string
	}{
		{
			name:        "creates the kubeconfig if it does not exist",
			userName:    "user",
			wantContext: "new",
			wantEntries: 1,
			wantOutput:  []string{"added new to kubeconfig", "logged into cluster new as user"},
		},
		{
			name:           "merges into an existing kubeconfig and keeps its current context",
			existing:       singleEntryConfig("existing"),
			wantContext:    "existing",
			wantEntries:    2,
			wantOutput:     []string{"added new to kubeconfig", "logged into cluster new"},
			unwantedOutput: []string{" as ", "organization"},
		},
		{
			name:        "switches the current context and sets the project",
			existing:    singleEntryConfig("existing"),
			toOrg:       "org",
			opts:        []LoginOption{SwitchCurrentContext(), WithProject("project")},
			wantContext: "new",
			wantEntries: 2,
			wantProject: "project",
			wantOutput:  []string{`switched to the organization "org"`, "added new to kubeconfig"},
		},
		{
			name:        "overwrites an entry with the same name",
			existing:    singleEntryConfig("new"),
			wantContext: "new",
			wantEntries: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			kubeconfigPath := filepath.Join(t.TempDir(), "kubeconfig")
			if tt.existing != nil {
				is.NoError(clientcmd.WriteToFile(*tt.existing, kubeconfigPath))
			}

			newConfig := singleEntryConfig("new")
			newConfig.AuthInfos["new"].Token = "new-token"
			out := &bytes.Buffer{}
			is.NoError(Login(format.NewWriter(out), newConfig, kubeconfigPath, tt.userName, tt.toOrg, tt.opts...))

			merged, err := clientcmd.LoadFromFile(kubeconfigPath)
			is.NoError(err)
			is.Equal(tt.wantContext, merged.CurrentContext)
			is.Len(merged.Clusters, tt.wantEntries)
			is.Len(merged.Contexts, tt.wantEntries)
			is.Len(merged.AuthInfos, tt.wantEntries)
			is.Equal("new-token", merged.AuthInfos["new"].Token)
			is.Equal(tt.wantProject, merged.Contexts["new"].Namespace)
			for _, want := range tt.wantOutput {
				is.Contains(out.String(), want)
			}
			for _, unwanted := range tt.unwantedOutput {
				is.NotContains(out.String(), unwanted)
			}
		})
	}
}
