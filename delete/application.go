package delete

import (
	"context"
	"errors"
	"fmt"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/gitinfo"
	"github.com/ninech/nctl/api/nctl"
	"github.com/ninech/nctl/internal/application"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type applicationCmd struct {
	resourceCmd
}

func (app *applicationCmd) Run(ctx context.Context, client *api.Client) error {
	ctx, cancel := context.WithTimeout(ctx, app.WaitTimeout)
	defer cancel()

	a := &apps.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: client.Project,
		},
	}
	gitAuthSecrets, err := findGitAuthSecrets(ctx, client, a)
	if err != nil {
		return err
	}
	staticEgresses, err := application.StaticEgresses(ctx, client, api.ObjectName(a))
	if err != nil {
		return fmt.Errorf("finding static egresses of app: %w", err)
	}

	d := app.newDeleter(a, apps.ApplicationKind)
	if err := d.deleteResource(ctx, client, app.WaitTimeout, app.Wait, app.Force); err != nil {
		return fmt.Errorf("error while deleting %s: %w", apps.ApplicationKind, err)
	}

	var deleteErrors []error
	for _, s := range gitAuthSecrets {
		if err := app.deleteGitAuthSecret(ctx, client, s); err != nil {
			deleteErrors = append(deleteErrors, err)
		}
	}
	for _, egress := range staticEgresses {
		if err := client.Delete(ctx, &egress); err != nil {
			deleteErrors = append(deleteErrors, err)
		}
	}

	return errors.Join(deleteErrors...)
}

type manualCheckError string

func (m manualCheckError) Error() string {
	return string(m)
}

func checkManuallyError(err error) error {
	return manualCheckError(fmt.Sprintf("%v. Please take care of deleting the secret manually", err))
}

func findGitAuthSecrets(ctx context.Context, client *api.Client, a *apps.Application) ([]corev1.Secret, error) {
	// we always add the default secret which we create if git credentials
	// have been given
	gitAuthSecrets := []corev1.Secret{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gitinfo.AuthSecretName(a),
			Namespace: a.Namespace,
		},
	}}
	// we need to check if a different nctl created secret is referenced as well
	if err := client.Get(ctx, api.ObjectName(a), a); err != nil {
		return nil, fmt.Errorf("can not get application %s in project %s: %w",
			a.Name,
			a.Namespace,
			err,
		)
	}
	if a.Spec.ForProvider.Git.Auth != nil && a.Spec.ForProvider.Git.Auth.FromSecret != nil {
		gitAuthSecrets = append(
			gitAuthSecrets,
			corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      a.Spec.ForProvider.Git.Auth.FromSecret.Name,
					Namespace: a.Namespace,
				},
			},
		)
	}

	return gitAuthSecrets, nil
}

// deleteGitAuthSecret tries to delete the passed git auth secret. It checks
// if the secret is referenced in any other application, before deleting it.
// It will only delete secrets which have been created by nctl itself.
func (app *applicationCmd) deleteGitAuthSecret(
	ctx context.Context,
	client *api.Client,
	secret corev1.Secret,
) error {
	if err := client.Get(ctx, api.ObjectName(&secret), &secret); err != nil {
		if kerrors.IsNotFound(err) {
			return nil
		}
		return checkManuallyError(
			fmt.Errorf("error when checking git auth secret %q for application", secret.Name),
		)
	}
	managedBy, exists := secret.Annotations[nctl.ManagedByAnnotation]
	if !exists || managedBy != nctl.Name {
		// the secret was not created by nctl, so we will not delete it
		return nil
	}

	appsList := &apps.ApplicationList{}
	if err := client.List(ctx, appsList, runtimeclient.InNamespace(client.Project)); err != nil {
		return checkManuallyError(
			fmt.Errorf(
				"error when checking for applications which might reference git authentication secret %q: %w",
				secret.Name,
				err,
			),
		)
	}
	for _, item := range appsList.Items {
		if item.Spec.ForProvider.Git.Auth != nil &&
			item.Spec.ForProvider.Git.Auth.FromSecret != nil &&
			item.Spec.ForProvider.Git.Auth.FromSecret.Name == secret.Name {
			app.Printf(
				"will not delete git auth secret %q as it is still referenced in application %q\n",
				secret.Name,
				item.Name,
			)
			return nil
		}
	}
	if err := client.Delete(ctx, &secret); err != nil {
		return checkManuallyError(
			fmt.Errorf("error when deleting git auth secret %q: %w", secret.Name, err),
		)
	}

	return nil
}
