package create

import (
	"context"
	"fmt"
	"strings"
	"time"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/auth"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type projectCmd struct {
	Name        string        `arg:"" default:"" help:"Name of the project. A random name is generated if omitted."`
	Wait        bool          `default:"true" help:"Wait until the project was fully created."`
	WaitTimeout time.Duration `default:"10m" help:"Duration to wait for project getting ready. Only relevant if wait is set."`
}

func (proj *projectCmd) Run(ctx context.Context, client *api.Client) error {
	cfg, err := auth.ReadConfig(client.KubeconfigPath, client.KubeconfigContext)
	if err != nil {
		if auth.IsConfigNotFoundError(err) {
			return auth.ReloginNeeded(err)
		}
		return err
	}

	p := newProject(proj.Name, cfg.Organization)
	fmt.Printf("Creating new project %s for organization %s", p.Name, cfg.Organization)
	c := newCreator(client, p, strings.ToLower(management.ProjectKind))
	ctx, cancel := context.WithTimeout(ctx, proj.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !proj.Wait {
		return nil
	}

	return c.wait(ctx, waitStage{
		objectList: &management.ProjectList{},
		onResult:   resourceAvailable,
	})
}

func newProject(name, namespace string) *management.Project {
	return &management.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getName(name),
			Namespace: namespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       management.ProjectKind,
			APIVersion: management.SchemeGroupVersion.String(),
		},
		Spec: management.ProjectSpec{},
	}
}
