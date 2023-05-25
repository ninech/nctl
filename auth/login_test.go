package auth

import (
	"context"
	"io"
	"log"
	"os"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

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
	// we run without the execPlugin, that would be something for an e2e test
	cmd := &LoginCmd{Organization: "test", ExecPlugin: false, APIURL: "https://" + apiHost, IssuerURL: "https://auth.example.org"}
	if err := cmd.Run(context.Background(), ""); err != nil {
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
