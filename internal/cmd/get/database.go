package get

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/database"
)

// DatabaseCmd is the shared base for the get sub-commands of databases.
// It adds the backup flags to [ServiceCmd].
//
// It has to be exported so that Kong initializes the embedded [format.Writer], see [format.Writer.BeforeApply].
type DatabaseCmd struct {
	ServiceCmd
	PrintBackupBucket      bool `help:"Print the URL of the bucket storing the backups of the database. Requires name to be set." xor:"print"`
	PrintBackupCredentials bool `help:"Print credentials for read access to the backups of the database. Requires name to be set." xor:"print"`
}

func (cmd *DatabaseCmd) run(ctx context.Context, client *api.Client, out *output, list resource.ManagedList, svc service) error {
	if cmd.Name == "" || !cmd.PrintBackupBucket && !cmd.PrintBackupCredentials {
		return cmd.ServiceCmd.run(ctx, client, out, list, svc)
	}
	if len(list.GetItems()) == 0 {
		return out.notFound(svc.kind, client.Project)
	}

	// the database can be in another project than the client's, e.g. with --all-projects
	db := list.GetItems()[0]
	bucket, err := backupBucketName(ctx, client, db.GetNamespace(), database.Ref(svc.kind, db.GetName()))
	if err != nil {
		return err
	}

	if cmd.PrintBackupBucket {
		url, err := backupBucketURL(ctx, client, db.GetNamespace(), bucket, "")
		if err != nil {
			return err
		}
		out.Println(url)
		return nil
	}

	user, err := backupBucketUser(ctx, client, db.GetNamespace(), bucket)
	if err != nil {
		return err
	}
	return cmd.printCredentials(ctx, client, user, out, nil)
}
