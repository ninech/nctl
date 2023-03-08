package get

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	iam "github.com/ninech/apis/iam/v1alpha1"
	"github.com/ninech/nctl/api"
)

type apiServiceAccountsCmd struct{}

func (asa *apiServiceAccountsCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	asaList := &iam.APIServiceAccountList{}

	if err := list(ctx, client, asaList, get.AllNamespaces); err != nil {
		return err
	}

	if len(asaList.Items) == 0 {
		printEmptyMessage(iam.APIServiceAccountKind, client.Namespace)
		return nil
	}

	switch get.Output {
	case full:
		return asa.print(asaList.Items, true)
	case noHeader:
		return asa.print(asaList.Items, false)
	}

	return nil
}

func (asa *apiServiceAccountsCmd) print(sas []iam.APIServiceAccount, header bool) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)

	if header {
		fmt.Fprintf(w, "%s\t%s\t%s\n", "NAME", "NAMESPACE", "ROLE")
	}

	for _, sa := range sas {
		fmt.Fprintf(w, "%s\t%s\t%s\n", sa.Name, sa.Namespace, sa.Spec.ForProvider.Role)
	}

	return w.Flush()
}
