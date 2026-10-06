package get

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type openSearchCmd struct {
	ServiceCmd
	PrintSnapshotBucket bool `help:"Print the URL of the snapshot bucket." xor:"print"`
}

func (cmd *openSearchCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	return get.listPrint(ctx, client, cmd, api.MatchName(cmd.Name))
}

func (cmd *openSearchCmd) list() client.ObjectList {
	return &storage.OpenSearchList{}
}

func (cmd *openSearchCmd) print(
	ctx context.Context,
	client *api.Client,
	list client.ObjectList,
	out *output,
) error {
	openSearchList, ok := list.(*storage.OpenSearchList)
	if !ok {
		return fmt.Errorf("expected %T, got %T", &storage.OpenSearchList{}, list)
	}

	if cmd.Name != "" && cmd.PrintSnapshotBucket && len(openSearchList.Items) > 0 {
		return cmd.printSnapshotBucket(ctx, client, &openSearchList.Items[0], out)
	}

	return cmd.run(ctx, client, out, openSearchList, service{
		kind:             storage.OpenSearchKind,
		connectionString: cmd.connectionString,
		printList:        cmd.printOpenSearchInstances,
		caCert: func(mg resource.Managed) (string, error) {
			os, ok := mg.(*storage.OpenSearch)
			if !ok {
				return "", fmt.Errorf("expected %T, got %T", &storage.OpenSearch{}, mg)
			}
			return os.Status.AtProvider.CACert, nil
		},
	})
}

// connectionString returns the public URL of the cluster with the basic auth credentials embedded.
func (cmd *openSearchCmd) connectionString(mg resource.Managed, user, password string) (string, error) {
	os, ok := mg.(*storage.OpenSearch)
	if !ok {
		return "", fmt.Errorf("expected %T, got %T", &storage.OpenSearch{}, mg)
	}

	if os.Status.AtProvider.URL == "" {
		return "", errors.New("no URL found, the service might not be ready yet")
	}

	u, err := url.Parse(string(os.Status.AtProvider.URL))
	if err != nil {
		return "", fmt.Errorf("unable to parse URL: %w", err)
	}
	u.User = url.UserPassword(user, password)

	return u.String(), nil
}

func (cmd *openSearchCmd) printOpenSearchInstances(
	resources resource.ManagedList,
	out *output,
	header bool,
) error {
	list, ok := resources.(*storage.OpenSearchList)
	if !ok {
		return fmt.Errorf("expected %T, got %T", &storage.OpenSearchList{}, resources)
	}

	if header {
		out.writeHeader(
			"NAME",
			"LOCATION",
			"VERSION",
			"PRIVATE URL",
			"PUBLIC URL",
			"MACHINE TYPE",
			"CLUSTER TYPE",
			"DISK SIZE",
			"HEALTH",
		)
	}

	for _, os := range list.Items {
		out.writeTabRow(
			os.Namespace,
			os.Name,
			string(os.Spec.ForProvider.Location),
			string(os.Spec.ForProvider.Version),
			string(os.Status.AtProvider.PrivateNetworkingURL),
			string(os.Status.AtProvider.URL),
			os.Spec.ForProvider.MachineType.String(),
			string(os.Spec.ForProvider.ClusterType),
			os.Status.AtProvider.DiskSize.String(),
			string(cmd.getClusterHealth(os.Status.AtProvider.ClusterHealth)),
		)
	}

	return out.tabWriter.Flush()
}

func (cmd *openSearchCmd) getClusterHealth(
	clusterHealth storage.OpenSearchClusterHealth,
) storage.OpenSearchHealthStatus {
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

func (cmd *openSearchCmd) printSnapshotBucket(
	ctx context.Context,
	client *api.Client,
	openSearch *storage.OpenSearch,
	out *output,
) error {
	bucketName := openSearch.Status.AtProvider.SnapshotsBucket.Name
	if bucketName == "" {
		return fmt.Errorf(
			"no snapshot bucket configured for OpenSearch instance %s",
			openSearch.Name,
		)
	}

	bucket := &storage.Bucket{}
	if err := client.Get(
		ctx,
		types.NamespacedName{Name: bucketName, Namespace: client.Project},
		bucket,
	); err != nil {
		return err
	}

	bucketURL := bucket.Status.AtProvider.PublicURL
	if bucketURL == "" {
		return fmt.Errorf("no URL found in ObjectsBucket %s status", bucketName)
	}

	out.Println(bucketURL)
	return nil
}
