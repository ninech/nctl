package create

import (
	"context"
	"time"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	iam "github.com/ninech/apis/iam/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type apiServiceAccountCmd struct {
	Name        string        `arg:"" default:"" help:"Name of the API Service Account. A random name is generated if omitted."`
	Wait        bool          `default:"true" help:"Wait until API Service Account is fully created."`
	WaitTimeout time.Duration `default:"10s" help:"Duration to wait for API Service Account getting ready. Only relevant if wait is set."`
}

func (asa *apiServiceAccountCmd) Run(ctx context.Context, client *api.Client) error {
	c := newCreator(asa.newAPIServiceAccount(client.Namespace), iam.APIServiceAccountKind, &iam.APIServiceAccountList{})
	ctx, cancel := context.WithTimeout(ctx, asa.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx, client); err != nil {
		return err
	}

	if !asa.Wait {
		return nil
	}

	return c.wait(ctx, client, resourceAvailable)
}

func (asa *apiServiceAccountCmd) newAPIServiceAccount(namespace string) *iam.APIServiceAccount {
	name := getName(asa.Name)
	return &iam.APIServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: iam.APIServiceAccountSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      name,
					Namespace: namespace,
				},
			},
		},
	}
}
