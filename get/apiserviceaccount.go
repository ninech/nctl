package get

import (
	"context"
	"fmt"

	iam "github.com/ninech/apis/iam/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type apiServiceAccountsCmd struct {
	resourceCmd
	PrintToken      bool `help:"Print the bearer token of the Account. Requires name to be set." default:"false"`
	PrintKubeconfig bool `help:"Print the kubeconfig of the Account. Requires name to be set." default:"false"`
}

const (
	tokenKey      = "token"
	kubeconfigKey = "kubeconfig"
)

func (asa *apiServiceAccountsCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	return get.listPrint(ctx, client, asa, api.MatchName(asa.Name))
}

func (asa *apiServiceAccountsCmd) list() client.ObjectList {
	return &iam.APIServiceAccountList{}
}

func (asa *apiServiceAccountsCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	asaList := list.(*iam.APIServiceAccountList)
	if len(asaList.Items) == 0 {
		out.printEmptyMessage(iam.APIServiceAccountKind, client.Project)
		return nil
	}

	if len(asa.Name) != 0 {
		if asa.PrintToken {
			return asa.printToken(ctx, client, &asaList.Items[0])
		}

		if asa.PrintKubeconfig {
			return asa.printKubeconfig(ctx, client, &asaList.Items[0])
		}
	}

	if asa.PrintToken || asa.PrintKubeconfig {
		return fmt.Errorf("name is not set, token or kubeconfig can only be printed for a single API Service Account")
	}

	switch out.Format {
	case full:
		return asa.printAsa(asaList.Items, out, true)
	case noHeader:
		return asa.printAsa(asaList.Items, out, false)
	case yamlOut:
		return format.PrettyPrintObjects(asaList.GetItems(), format.PrintOpts{})
	case jsonOut:
		return format.PrettyPrintObjects(
			asaList.GetItems(),
			format.PrintOpts{
				Format: format.OutputFormatTypeJSON,
				JSONOpts: format.JSONOutputOptions{
					PrintSingleItem: asa.Name != "",
				},
			})
	}

	return nil
}

func (asa *apiServiceAccountsCmd) printAsa(sas []iam.APIServiceAccount, out *output, header bool) error {
	if header {
		out.writeHeader("NAME", "ROLE")
	}

	for _, sa := range sas {
		out.writeTabRow(sa.Namespace, sa.Name, string(sa.Spec.ForProvider.Role))
	}

	return out.tabWriter.Flush()
}

func (asa *apiServiceAccountsCmd) printToken(ctx context.Context, client *api.Client, sa *iam.APIServiceAccount) error {
	token, err := getConnectionSecret(ctx, client, tokenKey, sa)
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", token)
	return nil
}

func (asa *apiServiceAccountsCmd) printKubeconfig(ctx context.Context, client *api.Client, sa *iam.APIServiceAccount) error {
	secret, err := client.GetConnectionSecret(ctx, sa)
	if err != nil {
		return fmt.Errorf("unable to get connection secret: %w", err)
	}

	kc, ok := secret.Data[kubeconfigKey]
	if !ok {
		return fmt.Errorf("secret of API Service Account %s has no kubeconfig", sa.Name)
	}

	fmt.Printf("%s", kc)

	return nil
}
