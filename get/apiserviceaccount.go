package get

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	iam "github.com/ninech/apis/iam/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
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

func (asa *apiServiceAccountsCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	return get.listPrint(ctx, client, asa, api.MatchName(asa.Name))
}

func (asa *apiServiceAccountsCmd) list() client.ObjectList {
	return &iam.APIServiceAccountList{}
}

func (asa *apiServiceAccountsCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	asaList := list.(*iam.APIServiceAccountList)
	if len(asaList.Items) == 0 {
		return out.printEmptyMessage(iam.APIServiceAccountKind, client.Project)
	}
	sa := &asaList.Items[0]

	if asa.printFlagSet() {
		if asa.Name == "" {
			return fmt.Errorf("name needs to be set to print service account information")
		}
		if err := asa.validPrintFlags(sa); err != nil {
			return err
		}
		if asa.PrintCredentials {
			return asa.printCredentials(ctx, client, sa, out)
		}
		key := ""
		switch sa.Spec.ForProvider.Version {
		default:
			if asa.PrintToken {
				key = iam.APIServiceAccountTokenKey
			}
			if asa.PrintKubeconfig {
				key = iam.APIServiceAccountKubeconfigKey
			}
		case iam.APIServiceAccountV2:
			if asa.PrintClientID {
				key = iam.APIServiceAccountIDKey
			}
			if asa.PrintClientSecret {
				key = iam.APIServiceAccountSecretKey
			}
			if asa.PrintTokenURL {
				key = iam.APIServiceAccountTokenURLKey
			}
			if asa.PrintKubeconfig {
				key = iam.APIServiceAccountKubeconfigKey
			}
		}
		return asa.printSecret(ctx, client, sa, key)
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

func (asa *apiServiceAccountsCmd) validPrintFlags(sa *iam.APIServiceAccount) error {
	switch sa.Spec.ForProvider.Version {
	case iam.APIServiceAccountV2:
		if asa.PrintToken {
			return fmt.Errorf("token printing is not supported for v2 APIServiceAccount")
		}
	default:
		if asa.PrintClientID || asa.PrintClientSecret || asa.PrintTokenURL {
			return fmt.Errorf("client_id/client_secret/token_url printing is not supported for v1 APIServiceAccount")
		}
	}
	return nil
}

func (asa *apiServiceAccountsCmd) printFlagSet() bool {
	return asa.PrintToken || asa.PrintKubeconfig || asa.PrintClientID || asa.PrintClientSecret || asa.PrintTokenURL || asa.PrintCredentials
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

func (asa *apiServiceAccountsCmd) printSecret(ctx context.Context, client *api.Client, sa *iam.APIServiceAccount, key string) error {
	data, err := getConnectionSecret(ctx, client, key, sa)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", data)
	return nil
}

func (asa *apiServiceAccountsCmd) printCredentials(ctx context.Context, client *api.Client, sa *iam.APIServiceAccount, out *output) error {
	data, err := getConnectionSecretMap(ctx, client, sa)
	if err != nil {
		return err
	}
	stringData := map[string]string{}
	for k, v := range data {
		// skip kubeconfig as it's multiline and it does not format nicely. We
		// have a separate flag to print it.
		if k == iam.APIServiceAccountKubeconfigKey {
			continue
		}
		stringData[k] = string(v)
	}

	switch out.Format {
	case full:
		out.writeTabRow("KEY", "VALUE")
		fallthrough
	case noHeader:
		for _, key := range slices.Sorted(maps.Keys(stringData)) {
			out.writeTabRow(key, stringData[key])
		}
		return out.tabWriter.Flush()
	case yamlOut:
		b, err := yaml.Marshal(stringData)
		if err != nil {
			return err
		}
		fmt.Print(string(b))
	case jsonOut:
		b, err := json.MarshalIndent(stringData, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	}
	return nil
}
