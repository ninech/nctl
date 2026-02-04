package get

import (
	"context"
	"fmt"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type bucketUserCmd struct {
	resourceCmd
	PrintCredentials bool `help:"Print the credentials of the BucketUser. Requires name to be set." xor:"cred"`
	PrintAccessKey   bool `help:"Print the access key of the BucketUser. Requires name to be set." xor:"access"`
	PrintSecretKey   bool `help:"Print the secret key of the BucketUser. Requires name to be set." xor:"secret"`
}

func (bu *bucketUserCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	return get.listPrint(ctx, client, bu, api.MatchName(bu.Name))
}

func (bu *bucketUserCmd) list() client.ObjectList {
	return &storage.BucketUserList{}
}

func (bu *bucketUserCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	bucketUserList := list.(*storage.BucketUserList)

	if len(bucketUserList.Items) == 0 {
		return out.printEmptyMessage(storage.BucketUserKind, client.Project)
	}

	user := &bucketUserList.Items[0]

	if bu.printFlagSet() {
		if bu.Name == "" {
			return fmt.Errorf("name needs to be set to print bucket user information")
		}

		if bu.PrintCredentials {
			return bu.printCredentials(ctx, client, user, out, nil)
		}

		key := ""
		if bu.PrintAccessKey {
			key = storage.BucketUserCredentialAccessKey
		}

		if bu.PrintSecretKey {
			key = storage.BucketUserCredentialSecretKey
		}
		return bu.printSecret(ctx, client, user, key, out)
	}

	switch out.Format {
	case full:
		return bu.printBucketUserInstances(bucketUserList.Items, out, true)
	case noHeader:
		return bu.printBucketUserInstances(bucketUserList.Items, out, false)
	case yamlOut:
		return format.PrettyPrintObjects(
			bucketUserList.GetItems(),
			format.PrintOpts{Out: &out.Writer},
		)
	case jsonOut:
		return format.PrettyPrintObjects(
			bucketUserList.GetItems(),
			format.PrintOpts{
				Out:    &out.Writer,
				Format: format.OutputFormatTypeJSON,
				JSONOpts: format.JSONOutputOptions{
					PrintSingleItem: bu.Name != "",
				},
			})
	}

	return nil
}

func (bu *bucketUserCmd) printBucketUserInstances(
	list []storage.BucketUser,
	out *output,
	header bool,
) error {
	if header {
		out.writeHeader("NAME", "LOCATION")
	}

	for _, bu := range list {
		out.writeTabRow(bu.Namespace, bu.Name, string(bu.Spec.ForProvider.Location))
	}

	return out.tabWriter.Flush()
}

func (bu *bucketUserCmd) printFlagSet() bool {
	return bu.PrintCredentials || bu.PrintAccessKey || bu.PrintSecretKey
}

func (bu *bucketUserCmd) printSecret(
	ctx context.Context,
	client *api.Client,
	user *storage.BucketUser,
	key string,
	out *output,
) error {
	data, err := getConnectionSecret(ctx, client, key, user)
	if err != nil {
		return err
	}
	out.Printf("%s\n", data)
	return nil
}
