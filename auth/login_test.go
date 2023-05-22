package auth

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path"
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

func TestLoginStaticToken(t *testing.T) {
	kubeconfig, err := os.CreateTemp("", "*-kubeconfig.yaml")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(kubeconfig.Name())

	os.Setenv(clientcmd.RecommendedConfigPathEnvVar, kubeconfig.Name())

	apiHost := "api.example.org"
	token := "faketoken"

	cmd := &LoginCmd{Organization: "test", APIURL: "https://" + apiHost, APIToken: token}
	if err := cmd.Run(context.Background(), ""); err != nil {
		t.Fatal(err)
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

	fmt.Printf("%s", b)
	checkConfig(t, kc, 1, "")

	if token != kc.AuthInfos[apiHost].Token {
		t.Fatalf("expected token to be %s, got %s", token, kc.AuthInfos[apiHost].Token)
	}

	if kc.AuthInfos[apiHost].Exec != nil {
		t.Fatalf("expected execConfig to be empty, got %v", kc.AuthInfos[apiHost].Exec)
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
	// we run without the execPlugin, that would be something for an e2e test
	cmd := &LoginCmd{Organization: "test", ExecPlugin: false, APIURL: "https://" + apiHost, IssuerURL: "https://auth.example.org"}
	if err := cmd.Run(context.Background(), ""); err != nil {
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
