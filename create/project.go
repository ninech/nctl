package create

import (
	"context"
	"fmt"
	"strings"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type projectCmd struct {
	common.ProjectCmd
	resourceCmd
	// DisplayName string `default:"" help:"Display Name of the project."`
}

func (proj *projectCmd) Run(ctx context.Context, client *api.Client) error {
	org, err := client.Organization()
	if err != nil {
		return err
	}

	p := newProject(org, proj)
	fmt.Printf("Creating new project %s for organization %s\n", p.Name, org)
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

func newProject(project string, cmd *projectCmd) *management.Project {
	displayName := ""
	if cmd.DisplayName != nil {
		displayName = *cmd.DisplayName
	}
	return &management.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getName(cmd.Name),
			Namespace: project,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       management.ProjectKind,
			APIVersion: management.SchemeGroupVersion.String(),
		},
		Spec: management.ProjectSpec{
			DisplayName: displayName,
		},
	}
}
