package get

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	iam "github.com/ninech/apis/iam/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type apiServiceAccountsCmd struct {
	Name            string `arg:"" help:"Name of the API Service Account to get. If omitted all in the project will be listed." default:""`
	PrintToken      bool   `help:"Print the bearer token of the Account. Requires name to be set." default:"false"`
	PrintKubeconfig bool   `help:"Print the kubeconfig of the Account. Requires name to be set." default:"false"`
}

const (
	tokenKey      = "token"
	kubeconfigKey = "kubeconfig"
)

func (asa *apiServiceAccountsCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	asaList := &iam.APIServiceAccountList{}

	if err := get.list(ctx, client, asaList, matchName(asa.Name)); err != nil {
		return err
	}

	if len(asaList.Items) == 0 {
		printEmptyMessage(os.Stdout, iam.APIServiceAccountKind, client.Project)
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

	switch get.Output {
	case full:
		return asa.print(asaList.Items, get, true)
	case noHeader:
		return asa.print(asaList.Items, get, false)
	case yamlOut:
		return format.PrettyPrintObjects(asaList.GetItems(), format.PrintOpts{})
	}

	return nil
}

func (asa *apiServiceAccountsCmd) print(sas []iam.APIServiceAccount, get *Cmd, header bool) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	if header {
		get.writeHeader(w, "NAME", "ROLE")
	}

	for _, sa := range sas {
		get.writeTabRow(w, sa.Namespace, sa.Name, string(sa.Spec.ForProvider.Role))
	}

	return w.Flush()
}

func (asa *apiServiceAccountsCmd) printToken(ctx context.Context, client *api.Client, sa *iam.APIServiceAccount) error {
	token, err := getConnectionSecret(ctx, client, "token", sa)
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
