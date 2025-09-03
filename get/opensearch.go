package get

import (
	"context"
	"fmt"
	"io"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type openSearchCmd struct {
	resourceCmd
	PrintPassword       bool `help:"Print the password of the OpenSearch BasicAuth User. Requires name to be set." xor:"print"`
	PrintUser           bool `help:"Print the name of the OpenSearch BasicAuth User. Requires name to be set." xor:"print"`
	PrintCACert         bool `help:"Print the ca certificate. Requires name to be set." xor:"print"`
	PrintSnapshotBucket bool `help:"Print the snapshot bucket name and the users who have read access to it." xor:"print"`
}

func (cmd *openSearchCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	return get.listPrint(ctx, client, cmd, api.MatchName(cmd.Name))
}

func (cmd *openSearchCmd) list() client.ObjectList {
	return &storage.OpenSearchList{}
}

func (cmd *openSearchCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	openSearchList, ok := list.(*storage.OpenSearchList)
	if !ok {
		return fmt.Errorf("expected %T, got %T", &storage.OpenSearchList{}, list)
	}
	if len(openSearchList.Items) == 0 {
		return out.printEmptyMessage(storage.OpenSearchKind, client.Project)
	}

	if cmd.Name != "" && cmd.PrintUser {
		return cmd.printSecret(out.writer, ctx, client, &openSearchList.Items[0], func(user, _ string) string { return user })
	}

	if cmd.Name != "" && cmd.PrintPassword {
		return cmd.printSecret(out.writer, ctx, client, &openSearchList.Items[0], func(_, pw string) string { return pw })
	}

	if cmd.Name != "" && cmd.PrintCACert {
		return printBase64(out.writer, openSearchList.Items[0].Status.AtProvider.CACert)
	}

	if cmd.Name != "" && cmd.PrintSnapshotBucket {
		return cmd.printSnapshotBucket(out.writer, &openSearchList.Items[0])
	}

	switch out.Format {
	case full:
		return cmd.printOpenSearchInstances(openSearchList.Items, out, true)
	case noHeader:
		return cmd.printOpenSearchInstances(openSearchList.Items, out, false)
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

func (cmd *openSearchCmd) printOpenSearchInstances(list []storage.OpenSearch, out *output, header bool) error {
	if header {
		out.writeHeader("NAME", "FQDN", "MACHINE TYPE", "CLUSTER TYPE", "DISK SIZE", "HEALTH")
	}

	for _, openSearch := range list {
		out.writeTabRow(
			openSearch.Namespace,
			openSearch.Name,
			openSearch.Status.AtProvider.FQDN,
			openSearch.Spec.ForProvider.MachineType.String(),
			string(openSearch.Spec.ForProvider.ClusterType),
			openSearch.Status.AtProvider.DiskSize.String(),
			string(cmd.getClusterHealth(openSearch.Status.AtProvider.ClusterHealth)),
		)
	}

	return out.tabWriter.Flush()
}

func (cmd *openSearchCmd) getClusterHealth(clusterHealth storage.OpenSearchClusterHealth) storage.OpenSearchHealthStatus {
	worstStatus := storage.OpenSearchHealthStatusGreen

	// If no indices, assume healthy
	if len(clusterHealth.Indices) == 0 {
		return worstStatus
	}
	// Determine the worst status of all indices
	for _, idx := range clusterHealth.Indices {
		switch idx.Status {
		case storage.OpenSearchHealthStatusRed:
			return idx.Status
		case storage.OpenSearchHealthStatusYellow:
			worstStatus = idx.Status
		}
	}

	return worstStatus
}

func (cmd *openSearchCmd) printSnapshotBucket(writer io.Writer, openSearch *storage.OpenSearch) error {
	bucketName := openSearch.Status.AtProvider.SnapshotsBucket.Name
	if bucketName == "" {
		bucketName = "No snapshot bucket found"
	}

	if _, err := fmt.Fprintln(writer, bucketName); err != nil {
		return err
	}

	return nil
}
