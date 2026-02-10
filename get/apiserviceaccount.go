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
	PrintCredentials  bool `help:"Print all credentials of the Account. Requires name to be set." xor:"print"`
	PrintToken        bool `help:"Print the bearer token of the Account. Requires name to be set. Only valid for v1 service accounts." xor:"print"`
	PrintKubeconfig   bool `help:"Print the kubeconfig of the Account. Requires name to be set. Only valid for v1 service accounts." xor:"print"`
	PrintClientID     bool `help:"Print the oauth2 client_id of the Account. Requires name to be set. Only valid for v2 service accounts." xor:"print"`
	PrintClientSecret bool `help:"Print the oauth2 client_secret of the Account. Requires name to be set. Only valid for v2 service accounts." xor:"print"`
	PrintTokenURL     bool `help:"Print the oauth2 token URL of the Account. Requires name to be set. Only valid for v2 service accounts." xor:"print"`
}

func (cmd *apiServiceAccountsCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	return get.listPrint(ctx, client, cmd, api.MatchName(cmd.Name))
}

func (cmd *apiServiceAccountsCmd) list() client.ObjectList {
	return &iam.APIServiceAccountList{}
}

func (cmd *apiServiceAccountsCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	asaList := list.(*iam.APIServiceAccountList)
	if len(asaList.Items) == 0 {
		return out.printEmptyMessage(iam.APIServiceAccountKind, client.Project)
	}
	sa := &asaList.Items[0]

	if cmd.printFlagSet() {
		if cmd.Name == "" {
			return fmt.Errorf("name needs to be set to print service account information")
		}
		if err := cmd.validPrintFlags(sa); err != nil {
			return err
		}
		if cmd.PrintCredentials {
			return cmd.printCredentials(
				ctx,
				client,
				sa,
				out,
				func(key string) bool { return key == iam.APIServiceAccountKubeconfigKey },
			)
		}
		key := ""
		switch sa.Spec.ForProvider.Version {
		default:
			if cmd.PrintToken {
				key = iam.APIServiceAccountTokenKey
			}
			if cmd.PrintKubeconfig {
				key = iam.APIServiceAccountKubeconfigKey
			}
		case iam.APIServiceAccountV2:
			if cmd.PrintClientID {
				key = iam.APIServiceAccountIDKey
			}
			if cmd.PrintClientSecret {
				key = iam.APIServiceAccountSecretKey
			}
			if cmd.PrintTokenURL {
				key = iam.APIServiceAccountTokenURLKey
			}
			if cmd.PrintKubeconfig {
				key = iam.APIServiceAccountKubeconfigKey
			}
		}
		return cmd.printSecret(ctx, client, sa, key, out)
	}

	switch out.Format {
	case full:
		return cmd.printAsa(asaList.Items, out, true)
	case noHeader:
		return cmd.printAsa(asaList.Items, out, false)
	case yamlOut:
		return format.PrettyPrintObjects(asaList.GetItems(), format.PrintOpts{Out: &out.Writer})
	case jsonOut:
		return format.PrettyPrintObjects(
			asaList.GetItems(),
			format.PrintOpts{
				Out:    &out.Writer,
				Format: format.OutputFormatTypeJSON,
				JSONOpts: format.JSONOutputOptions{
					PrintSingleItem: cmd.Name != "",
				},
			})
	}

	return nil
}

func (cmd *apiServiceAccountsCmd) validPrintFlags(sa *iam.APIServiceAccount) error {
	switch sa.Spec.ForProvider.Version {
	case iam.APIServiceAccountV2:
		if cmd.PrintToken {
			return fmt.Errorf("token printing is not supported for v2 APIServiceAccount")
		}
	default:
		if cmd.PrintClientID || cmd.PrintClientSecret || cmd.PrintTokenURL {
			return fmt.Errorf(
				"client_id/client_secret/token_url printing is not supported for v1 APIServiceAccount",
			)
		}
	}
	return nil
}

func (cmd *apiServiceAccountsCmd) printFlagSet() bool {
	return cmd.PrintToken || cmd.PrintKubeconfig || cmd.PrintClientID || cmd.PrintClientSecret ||
		cmd.PrintTokenURL ||
		cmd.PrintCredentials
}

func (cmd *apiServiceAccountsCmd) printAsa(
	sas []iam.APIServiceAccount,
	out *output,
	header bool,
) error {
	if header {
		out.writeHeader("NAME", "ROLE")
	}

	for _, sa := range sas {
		out.writeTabRow(sa.Namespace, sa.Name, string(sa.Spec.ForProvider.Role))
	}

	return out.tabWriter.Flush()
}

func (cmd *apiServiceAccountsCmd) printSecret(
	ctx context.Context,
	client *api.Client,
	sa *iam.APIServiceAccount,
	key string,
	out *output,
) error {
	data, err := getConnectionSecret(ctx, client, key, sa)
	if err != nil {
		return err
	}
	out.Printf("%s\n", data)
	return nil
}
