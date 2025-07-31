package get

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type openSearchCmd struct {
	resourceCmd
	PrintToken bool `help:"Print the bearer token of the Account. Requires name to be set." default:"false"`

	out io.Writer
}

func (cmd *openSearchCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	cmd.out = defaultOut(cmd.out)

	openSearchList := &storage.OpenSearchList{}

	if err := get.list(ctx, client, openSearchList, api.MatchName(cmd.Name)); err != nil {
		return err
	}

	if len(openSearchList.Items) == 0 {
		get.printEmptyMessage(cmd.out, storage.OpenSearchKind, client.Project)
		return nil
	}

	if cmd.Name != "" && cmd.PrintToken {
		return cmd.printPassword(ctx, client, &openSearchList.Items[0])
	}

	switch get.Output {
	case full:
		return cmd.printOpenSearchInstances(openSearchList.Items, get, true)
	case noHeader:
		return cmd.printOpenSearchInstances(openSearchList.Items, get, false)
	case yamlOut:
		return format.PrettyPrintObjects(openSearchList.GetItems(), format.PrintOpts{})
	case jsonOut:
		return format.PrettyPrintObjects(
			openSearchList.GetItems(),
			format.PrintOpts{
				Format: format.OutputFormatTypeJSON,
				JSONOpts: format.JSONOutputOptions{
					PrintSingleItem: cmd.Name != "",
				},
			})
	}

	return nil
}

func (cmd *openSearchCmd) printOpenSearchInstances(list []storage.OpenSearch, get *Cmd, header bool) error {
	w := tabwriter.NewWriter(cmd.out, 0, 0, 7, ' ', 0)

	if header {
		get.writeHeader(w, "NAME", "FQDN", "TLS", "MACHINE TYPE")
	}

	for _, openSearch := range list {
		get.writeTabRow(w, openSearch.Namespace, openSearch.Name, openSearch.Status.AtProvider.FQDN, "true", openSearch.Spec.ForProvider.MachineType.String())
	}

	return w.Flush()
}

func (cmd *openSearchCmd) printPassword(ctx context.Context, client *api.Client, openSearch *storage.OpenSearch) error {
	pw, err := getConnectionSecret(ctx, client, storage.OpenSearchUser, openSearch)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.out, pw)
	return nil
}
