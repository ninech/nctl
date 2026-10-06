package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/kubeconfig"

	"k8s.io/apimachinery/pkg/types"
)

type ClusterCmd struct {
	format.Writer `hidden:""`
	Name          string `arg:"" help:"Name of the cluster to authenticate with. Also accepts 'name/project' format."`
	ExecPlugin    bool   `help:"Automatically run exec plugin after writing the kubeconfig."`
}

func (a *ClusterCmd) Run(ctx context.Context, client *api.Client) error {
	name, err := clusterName(a.Name, client.Project)
	if err != nil {
		return err
	}

	return kubeconfig.LoginCluster(ctx, client, a.Writer, name, a.ExecPlugin)
}

func clusterName(name, project string) (types.NamespacedName, error) {
	parts := strings.Split(name, "/")
	if len(parts) == 2 {
		name = parts[0]
		project = parts[1]
	}

	if project == "" {
		return types.NamespacedName{}, fmt.Errorf("project cannot be empty")
	}

	return types.NamespacedName{Name: name, Namespace: project}, nil
}
