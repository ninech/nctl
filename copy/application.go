package copy

import (
	"context"
	"fmt"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/util"
	"github.com/ninech/nctl/internal/format"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type applicationCmd struct {
	resourceCmd
	Start     bool `help:"Automatically start copied app."`
	CopyHosts bool `help:"Also copy hosts of the old app."`
}

func (app *applicationCmd) Run(ctx context.Context, client *api.Client) error {
	newApp, err := app.newCopy(ctx, client)
	if err != nil {
		return fmt.Errorf("unable to copy app: %w", err)
	}

	if err := client.Create(ctx, newApp); err != nil {
		return fmt.Errorf("unable to create Application %q: %w", newApp.GetName(), err)
	}

	app.printCopyMessage(client, newApp)
	return nil
}

func (app *applicationCmd) targetNamespace(client *api.Client) string {
	if app.TargetProject != "" {
		return app.TargetProject
	}
	return client.Project
}

func (app *applicationCmd) newCopy(ctx context.Context, client *api.Client) (*apps.Application, error) {
	oldApp := &apps.Application{}
	if err := client.Get(ctx, client.Name(app.Name), oldApp); err != nil {
		return nil, err
	}
	newApp := &apps.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getName(app.TargetName),
			Namespace: app.targetNamespace(client),
		},
		Spec: oldApp.Spec,
	}
	newApp.Spec.ForProvider.Paused = !app.Start
	if !app.CopyHosts {
		newApp.Spec.ForProvider.Hosts = []string{}
	}
	if err := app.copyGitAuth(ctx, client, oldApp, newApp); err != nil {
		return nil, err
	}
	return newApp, nil
}

func (app *applicationCmd) printCopyMessage(client *api.Client, newApp *apps.Application) {
	format.PrintSuccessf("🏗", "Application %q in project %q has been copied to %q in project %q.",
		app.Name, client.Project, newApp.Name, app.targetNamespace(client))
	msg := ""
	if !app.CopyHosts {
		msg += "\nCustom hosts have not been copied and need to be migrated manually.\n"
	}
	if !app.Start {
		msg += "\nNote that the app is paused and you need to unpause it to make it available.\n"
	}
	if msg != "" {
		fmt.Println(msg)
	}
}

func (app *applicationCmd) copyGitAuth(ctx context.Context, client *api.Client, oldApp, newApp *apps.Application) error {
	if oldApp.Spec.ForProvider.Git.Auth == nil || oldApp.Spec.ForProvider.Git.Auth.FromSecret == nil {
		return nil
	}

	secret := &corev1.Secret{}
	if err := client.Get(ctx, client.Name(oldApp.Spec.ForProvider.Git.Auth.FromSecret.Name), secret); err != nil {
		return err
	}
	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.GitAuthSecretName(newApp),
			Namespace: newApp.Namespace,
			Annotations: map[string]string{
				util.ManagedByAnnotation: util.NctlName,
			},
		},
		Data: secret.Data,
	}
	if err := client.Create(ctx, newSecret); err != nil {
		if !kerrors.IsAlreadyExists(err) {
			return err
		}
	}

	return nil
}
