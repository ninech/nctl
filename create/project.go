package create

import (
	"context"
	"strings"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type projectCmd struct {
	resourceCmd
	DisplayName string `default:"" help:"Display Name of the project."`
}

func (cmd *projectCmd) Run(ctx context.Context, client *api.Client) error {
	org, err := client.Organization()
	if err != nil {
		return err
	}

	p := newProject(cmd.Name, org, cmd.DisplayName)
	c := cmd.newCreator(client, p, strings.ToLower(management.ProjectKind))
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !cmd.Wait {
		return nil
	}

	return c.wait(ctx, waitStage{
		Writer:     cmd.Writer,
		objectList: &management.ProjectList{},
		onResult:   resourceAvailable,
	})
}

func newProject(name, project, displayName string) *management.Project {
	return &management.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getName(name),
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
