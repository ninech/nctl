package copy

import (
	"context"
	"fmt"
	"strings"

	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/api/gitinfo"
	"github.com/ninech/nctl/api/nctl"
	"github.com/ninech/nctl/api/util"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type applicationCmd struct {
	resourceCmd
	Start              bool `help:"Automatically start copied app."`
	CopyHosts          bool `help:"Also copy hosts of the old app."`
	staticEgressCopied bool
}

func (cmd *applicationCmd) Run(ctx context.Context, client *api.Client) error {
	newApp, err := cmd.newCopy(ctx, client)
	if err != nil {
		return fmt.Errorf("unable to copy app: %w", err)
	}

	if err := client.Create(ctx, newApp); err != nil {
		return fmt.Errorf("unable to create Application %q: %w", newApp.GetName(), err)
	}

	cmd.printCopyMessage(client, newApp)
	return nil
}

func (cmd *applicationCmd) targetNamespace(client *api.Client) string {
	if cmd.TargetProject != "" {
		return cmd.TargetProject
	}
	return client.Project
}

func (cmd *applicationCmd) newCopy(
	ctx context.Context,
	client *api.Client,
) (*apps.Application, error) {
	oldApp := &apps.Application{}
	if err := client.Get(ctx, client.Name(cmd.Name), oldApp); err != nil {
		return nil, err
	}
	newApp := &apps.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      getName(cmd.TargetName),
			Namespace: cmd.targetNamespace(client),
		},
		Spec: oldApp.Spec,
	}
	newApp.Spec.ForProvider.Paused = !cmd.Start
	if !cmd.CopyHosts {
		newApp.Spec.ForProvider.Hosts = []string{}
	}
	if err := cmd.copyGitAuth(ctx, client, oldApp, newApp); err != nil {
		return nil, fmt.Errorf("copying git auth of app: %w", err)
	}
	if err := cmd.copyStaticEgress(ctx, client, oldApp, newApp); err != nil {
		return nil, fmt.Errorf("copying static egress of app: %w", err)
	}
	return newApp, nil
}

func (cmd *applicationCmd) printCopyMessage(client *api.Client, newApp *apps.Application) {
	cmd.Successf("🏗", "Application %q in project %q has been copied to %q in project %q.",
		cmd.Name, client.Project, newApp.Name, cmd.targetNamespace(client))

	msg := strings.Builder{}
	if !cmd.CopyHosts {
		msg.WriteString("\nCustom hosts have not been copied and need to be migrated manually.\n")
	}
	if !cmd.Start {
		msg.WriteString("\nNote that the app is paused and you need to unpause it to make it available.\n")
	}
	if cmd.staticEgressCopied {
		msg.WriteString("\nStatic egress has been copied to the new app. Note that the new static egress will get a new IP assigned.\n")
	}
	if msg.Len() > 0 {
		cmd.Println(msg.String())
	}
}

func (cmd *applicationCmd) copyGitAuth(
	ctx context.Context,
	client *api.Client,
	oldApp, newApp *apps.Application,
) error {
	if oldApp.Spec.ForProvider.Git.Auth == nil ||
		oldApp.Spec.ForProvider.Git.Auth.FromSecret == nil {
		return nil
	}

	secret := &corev1.Secret{}
	if err := client.Get(
		ctx,
		client.Name(oldApp.Spec.ForProvider.Git.Auth.FromSecret.Name),
		secret,
	); err != nil {
		return err
	}
	newSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gitinfo.AuthSecretName(newApp),
			Namespace: newApp.Namespace,
			Annotations: map[string]string{
				nctl.ManagedByAnnotation: nctl.Name,
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

func (cmd *applicationCmd) copyStaticEgress(
	ctx context.Context,
	client *api.Client,
	oldApp, newApp *apps.Application,
) error {
	egresses, err := util.ApplicationStaticEgresses(ctx, client, api.ObjectName(oldApp))
	if err != nil {
		return err
	}
	if len(egresses) == 0 {
		return nil
	}

	newEgress := &networking.StaticEgress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      newApp.Name,
			Namespace: newApp.Namespace,
		},
		Spec: networking.StaticEgressSpec{
			ForProvider: networking.StaticEgressParameters{
				Target: meta.LocalTypedReference{
					LocalReference: meta.LocalReference{
						Name: newApp.Name,
					},
					GroupKind: metav1.GroupKind{
						Group: apps.Group,
						Kind:  apps.ApplicationKind,
					},
				},
			},
		},
	}
	if err := client.Create(ctx, newEgress); err != nil {
		return err
	}
	cmd.staticEgressCopied = true
	return nil
}
