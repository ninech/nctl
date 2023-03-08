package auth

import (
	"context"
	"io"
	"log"
	"os"
	"testing"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const newKubeconfig = `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://new.example.org
  name: new
users:
- name: new
current-context: new
contexts:
- context:
  name: new
`

const existingKubeconfig = `
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://existing.example.org
  name: existing
users:
- name: existing
current-context: existing
contexts:
- context:
  name: existing
`

func TestClusterCmd(t *testing.T) {
	// write our "existing" kubeconfig to a temp kubeconfig
	kubeconfig, err := os.CreateTemp("", "*-kubeconfig.yaml")
	if err != nil {
		log.Fatal(err)
	}
	defer os.Remove(kubeconfig.Name())

	if err := os.WriteFile(kubeconfig.Name(), []byte(existingKubeconfig), os.ModePerm); err != nil {
		t.Fatal(err)
	}

	// prepare a fake client with some static objects (a KubernetesCluster and
	// a Secret) that the cluster cmd expects.
	scheme, err := api.NewScheme()
	if err != nil {
		t.Fatal(err)
	}

	secret := clusterSecret()
	cluster := newCluster(secret)
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(secret, cluster).Build()

	// we run without the execPlugin, that would be something for an e2e test
	cmd := &ClusterCmd{Name: ContextName(cluster), ExecPlugin: false}
	if err := cmd.Run(context.TODO(), &api.Client{WithWatch: client, KubeconfigPath: kubeconfig.Name()}); err != nil {
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

	checkConfig(t, merged, 2, "new")
}

func clusterSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Data: map[string][]byte{
			kubeconfigSecretKey: []byte(newKubeconfig),
		},
	}
}

func newCluster(secret *corev1.Secret) *infrastructure.KubernetesCluster {
	return &infrastructure.KubernetesCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: infrastructure.KubernetesClusterSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
			},
		},
	}
}
