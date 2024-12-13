package auth

import (
	"context"
	"io"
	"log"
	"os"
	"path"
	"testing"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

type fakeTokenGetter struct{}

func (f *fakeTokenGetter) GetTokenString(ctx context.Context, issuerURL, clientID string, usePKCE bool) (string, error) {
	return test.FakeJWTToken, nil
}

func checkErrorRequire(t *testing.T, err error, expectError bool, expectedErrMsg string) {
	t.Helper()
	if expectError {
		require.Error(t, err, "expected an error but got none")
		require.EqualError(t, err, expectedErrMsg, "unexpected error message")
	} else {
		require.NoError(t, err, "expected no error but got one")
	}
}

func TestLoginCmd(t *testing.T) {
	// write our "existing" kubeconfig to a temp kubeconfig
	kubeconfig, err := os.CreateTemp("", "*-kubeconfig.yaml")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(kubeconfig.Name())

	if err := os.WriteFile(kubeconfig.Name(), []byte(existingKubeconfig), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	os.Setenv(clientcmd.RecommendedConfigPathEnvVar, kubeconfig.Name())

	apiHost := "api.example.org"
	tk := &fakeTokenGetter{}
	cmd := &LoginCmd{
		APIURL:                      "https://" + apiHost,
		IssuerURL:                   "https://auth.example.org",
		ForceInteractiveEnvOverride: true,
	}
	if err := cmd.Run(context.Background(), "", tk); err != nil {
		t.Fatal(err)
	}

	// read out the kubeconfig again to test the contents
	b, err := io.ReadAll(kubeconfig)
	if err != nil {
		t.Fatal(err)
	}

	merged, err := clientcmd.Load(b)
	if err != nil {
		t.Fatal(err)
	}

	checkConfig(t, merged, 2, "existing")
}

func TestLoginStaticToken(t *testing.T) {
	apiHost := "api.example.org"

	tests := []struct {
		name           string
		cmd            *LoginCmd
		tk             api.TokenGetter
		wantToken      string
		wantErr        bool
		wantErrMessage string
	}{
		{
			name:      "interactive environment with token",
			cmd:       &LoginCmd{APIURL: "https://" + apiHost, APIToken: test.FakeJWTToken, Organization: "test", ForceInteractiveEnvOverride: true},
			tk:        &fakeTokenGetter{},
			wantToken: test.FakeJWTToken,
		},
		{
			name:      "non-interactive environment with token",
			cmd:       &LoginCmd{APIURL: "https://" + apiHost, APIToken: test.FakeJWTToken, Organization: "test", ForceInteractiveEnvOverride: false},
			tk:        &fakeTokenGetter{},
			wantToken: test.FakeJWTToken,
		},
		{
			name:           "non-interactive environment with empty token",
			cmd:            &LoginCmd{APIURL: "https://" + apiHost, APIToken: "", Organization: "test", ForceInteractiveEnvOverride: false},
			wantErr:        true,
			wantErrMessage: ErrNonInteractiveEnvironmentEmptyToken,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			kubeconfig, err := os.CreateTemp("", "*-kubeconfig.yaml")
			if err != nil {
				log.Fatal(err)
			}
			defer os.Remove(kubeconfig.Name())
			os.Setenv(clientcmd.RecommendedConfigPathEnvVar, kubeconfig.Name())

			err = tt.cmd.Run(context.Background(), "", tt.tk)
			checkErrorRequire(t, err, tt.wantErr, tt.wantErrMessage)

			if tt.wantErr {
				return
			}

			// read out the kubeconfig again to test the contents
			b, err := io.ReadAll(kubeconfig)
			if err != nil {
				t.Fatal(err)
			}

			kc, err := clientcmd.Load(b)
			if err != nil {
				t.Fatal(err)
			}

			checkConfig(t, kc, 1, "")

			if tt.wantToken != kc.AuthInfos[apiHost].Token {
				t.Fatalf("expected token to be %s, got %s", tt.wantToken, kc.AuthInfos[apiHost].Token)
			}

			if kc.AuthInfos[apiHost].Exec != nil {
				t.Fatalf("expected execConfig to be empty, got %v", kc.AuthInfos[apiHost].Exec)
			}
		})
	}
}

func TestLoginCmdWithoutExistingKubeconfig(t *testing.T) {
	dir, err := os.MkdirTemp("", "nctl-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	kubeconfig := path.Join(dir, "test-kubeconfig.yaml")
	os.Setenv(clientcmd.RecommendedConfigPathEnvVar, kubeconfig)

	apiHost := "api.example.org"
	cmd := &LoginCmd{
		APIURL:                      "https://" + apiHost,
		IssuerURL:                   "https://auth.example.org",
		ForceInteractiveEnvOverride: true,
	}
	tk := &fakeTokenGetter{}
	if err := cmd.Run(context.Background(), "", tk); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(kubeconfig)
	if err != nil {
		t.Fatal(err)
	}

	// read out the kubeconfig again to test the contents
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}

	kc, err := clientcmd.Load(b)
	if err != nil {
		t.Fatal(err)
	}

	checkConfig(t, kc, 1, apiHost)
}

func checkConfig(t *testing.T, cfg *clientcmdapi.Config, expectedLen int, expectedContext string) {
	if len(cfg.Clusters) != expectedLen {
		t.Fatalf("expected config to contain %v clusters, got %v", expectedLen, len(cfg.Clusters))
	}

	if len(cfg.Contexts) != expectedLen {
		t.Fatalf("expected config to contain %v contexts, got %v", expectedLen, len(cfg.Contexts))
	}

	if len(cfg.AuthInfos) != expectedLen {
		t.Fatalf("expected config to contain %v authinfos, got %v", expectedLen, len(cfg.AuthInfos))
	}

	if cfg.CurrentContext != expectedContext {
		t.Fatalf("expected config current-context to be %q, got %q", expectedContext, cfg.CurrentContext)
	}
}
