package get

import (
	"context"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type bucketUserCmd struct {
	resourceCmd
	PrintCredentials bool `help:"Print the credentials of the BucketUser. Requires name to be set." xor:"print" aliases:"print-bucket-user"`
}

func (cmd *bucketUserCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	return get.listPrint(ctx, client, cmd, api.MatchName(cmd.Name))
}

func (cmd *bucketUserCmd) list() client.ObjectList {
	return &storage.BucketUserList{}
}

func (cmd *bucketUserCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	bucketUserList := list.(*storage.BucketUserList)

	if len(bucketUserList.Items) == 0 {
		return out.printEmptyMessage(storage.BucketUserKind, client.Project)
	}

	if cmd.Name != "" && cmd.PrintCredentials {
		return cmd.printSecret(out.writer, ctx, client, bucketUserList.GetItems()[0], func(k, v string) string { return k + ": " + v })
	}

	switch out.Format {
	case full:
		return cmd.printBucketUserInstances(bucketUserList.Items, out, true)
	case noHeader:
		return cmd.printBucketUserInstances(bucketUserList.Items, out, false)
	case yamlOut:
		return format.PrettyPrintObjects(bucketUserList.GetItems(), format.PrintOpts{Out: out.writer})
	case jsonOut:
		return format.PrettyPrintObjects(
			bucketUserList.GetItems(),
			format.PrintOpts{
				Out:    out.writer,
				Format: format.OutputFormatTypeJSON,
				JSONOpts: format.JSONOutputOptions{
					PrintSingleItem: cmd.Name != "",
				},
			})
	}

	return nil
}

func (cmd *bucketUserCmd) printBucketUserInstances(list []storage.BucketUser, out *output, header bool) error {
	if header {
		out.writeHeader("NAME", "LOCATION", "BACKEND VERSION")
	}

	for _, bu := range list {
		out.writeTabRow(bu.Namespace, bu.Name, string(bu.Spec.ForProvider.Location), string(bu.Spec.ForProvider.BackendVersion))
	}

	return out.tabWriter.Flush()
}
