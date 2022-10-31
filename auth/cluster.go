package auth

import (
	"context"
	"fmt"
	"strings"

	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	"github.com/ninech/nctl/api"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
)

type ClusterCmd struct {
	Name       string `arg:"" help:"Name of the cluster to authenticate with. Also accepts 'name/namespace' format."`
	ExecPlugin bool   `help:"Automatically run exec plugin after writing the kubeconfig."`
}

const kubeconfigSecretKey = "kubeconfig"

func (a *ClusterCmd) Run(client *api.Client) error {
	name, err := clusterName(a.Name, client.Namespace)
	if err != nil {
		return err
	}

	cluster := &infrastructure.KubernetesCluster{}
	if err := client.Get(context.Background(), name, cluster); err != nil {
		return err
	}

	if cluster.Spec.WriteConnectionSecretToReference == nil {
		return fmt.Errorf("cluster connection secret reference is empty")
	}

	secret := &corev1.Secret{}
	if err := client.Get(context.Background(), types.NamespacedName{
		Name:      cluster.Spec.WriteConnectionSecretToReference.Name,
		Namespace: cluster.Spec.WriteConnectionSecretToReference.Namespace,
	}, secret); err != nil {
		return err
	}

	cfg, err := clientcmd.Load(secret.Data[kubeconfigSecretKey])
	if err != nil {
		return err
	}

	if err := login(cfg, client.KubeconfigPath, runExecPlugin(a.ExecPlugin)); err != nil {
		return fmt.Errorf("error logging in to cluster %s: %w", name, err)
	}

	return nil
}

func clusterName(name, namespace string) (types.NamespacedName, error) {
	parts := strings.Split(name, "/")
	if len(parts) == 2 {
		name = parts[0]
		namespace = parts[1]
	}

	if namespace == "" {
		return types.NamespacedName{}, fmt.Errorf("namespace cannot be empty")
	}

	return types.NamespacedName{Name: name, Namespace: namespace}, nil
}

func ContextName(cluster *infrastructure.KubernetesCluster) string {
	return fmt.Sprintf("%s/%s", cluster.Name, cluster.Namespace)
}
