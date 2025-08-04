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
	PrintPassword bool `help:"Print the password of the OpenSearch BasicAuth User. Requires name to be set." xor:"print"`
	PrintUser     bool `help:"Print the name of the OpenSearch BasicAuth User. Requires name to be set." xor:"print"`
	PrintPort     bool `help:"Print the port of the OpenSearch instance. Requires name to be set." xor:"print"`

	out io.Writer
}

func (cmd *openSearchCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	cmd.out = defaultOut(cmd.out)

	if cmd.Name != "" && cmd.PrintUser {
		fmt.Fprintln(cmd.out, storage.OpenSearchUser)
		return nil
	}

	if cmd.Name != "" && cmd.PrintPort {
		fmt.Fprintln(cmd.out, storage.OpenSearchHTTPPort)
		return nil
	}

	openSearchList := &storage.OpenSearchList{}
	if err := get.list(ctx, client, openSearchList, api.MatchName(cmd.Name)); err != nil {
		return err
	}
	if len(openSearchList.Items) == 0 {
		get.printEmptyMessage(cmd.out, storage.OpenSearchKind, client.Project)
		return nil
	}

	if cmd.Name != "" && cmd.PrintPassword {
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
	w := tabwriter.NewWriter(cmd.out, 0, 0, 4, ' ', 0)

	if header {
		get.writeHeader(w, "NAME", "FQDN", "PORT", "MACHINE TYPE", "CLUSTER TYPE", "DISK SIZE", "HEALTH")
	}

	for _, openSearch := range list {
		get.writeTabRow(
			w,
			openSearch.Namespace,
			openSearch.Name,
			openSearch.Status.AtProvider.FQDN,
			fmt.Sprintf("%d", storage.OpenSearchHTTPPort),
			openSearch.Spec.ForProvider.MachineType.String(),
			string(openSearch.Spec.ForProvider.ClusterType),
			openSearch.Status.AtProvider.DiskSize.String(),
			cmd.getClusterHealth(openSearch.Status.AtProvider.ClusterHealth),
		)
	}

	return w.Flush()
}

func (cmd *openSearchCmd) getClusterHealth(clusterHealth storage.OpenSearchClusterHealth) string {
	worstStatus := storage.OpenSearchHealthStatusGreen

	// If no indices, assume healthy
	if len(clusterHealth.Indices) == 0 {
		return string(worstStatus)
	}
	// Determine the worst status of all indices
	for _, idx := range clusterHealth.Indices {
		switch idx.Status {
		case storage.OpenSearchHealthStatusRed:
			worstStatus = storage.OpenSearchHealthStatusRed
		case storage.OpenSearchHealthStatusYellow:
			if worstStatus != storage.OpenSearchHealthStatusRed {
				worstStatus = storage.OpenSearchHealthStatusYellow
			}
		}
	}

	return string(worstStatus)
}

func (cmd *openSearchCmd) printPassword(ctx context.Context, client *api.Client, openSearch *storage.OpenSearch) error {
	pw, err := getConnectionSecret(ctx, client, storage.OpenSearchUser, openSearch)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.out, pw)
	return nil
}
