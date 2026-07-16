package get

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"time"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/database"
	"github.com/ninech/nctl/internal/format"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/duration"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type databaseBackupCmd struct {
	resourceCmd
	Source           string `help:"Only show backups of this database, in the form <kind>/<name> (e.g. postgresdatabase/mydb)."`
	PrintURL         bool   `help:"Print the URL of the backup in its bucket. Requires name to be set." xor:"print"`
	PrintCredentials bool   `help:"Print credentials for read access to the backup's bucket. Requires name to be set." xor:"print"`
}

func (cmd *databaseBackupCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	return get.listPrint(ctx, client, cmd, api.MatchName(cmd.Name))
}

func (cmd *databaseBackupCmd) list() runtimeclient.ObjectList {
	return &storage.DatabaseBackupList{}
}

func (cmd *databaseBackupCmd) print(ctx context.Context, client *api.Client, list runtimeclient.ObjectList, out *output) error {
	backupList, ok := list.(*storage.DatabaseBackupList)
	if !ok {
		return fmt.Errorf("expected %T, got %T", &storage.DatabaseBackupList{}, list)
	}
	if cmd.Source != "" {
		source, err := database.ParseRef(cmd.Source)
		if err != nil {
			return err
		}
		backupList.Items = slices.DeleteFunc(backupList.Items, func(b storage.DatabaseBackup) bool {
			return !database.SameRef(b.Spec.ForProvider.Source, source)
		})
	}
	if len(backupList.Items) == 0 {
		return out.notFound(storage.DatabaseBackupKind, client.Project)
	}

	if cmd.Name != "" && cmd.PrintURL {
		backup := backupList.Items[0]
		url, err := backupBucketURL(ctx, client, backup.Spec.ForProvider.Bucket.Name, backup.Status.AtProvider.Path)
		if err != nil {
			return err
		}
		out.Println(url)
		return nil
	}

	if cmd.Name != "" && cmd.PrintCredentials {
		user, err := backupBucketUser(ctx, client, backupList.Items[0].Spec.ForProvider.Bucket.Name)
		if err != nil {
			return err
		}
		return cmd.printCredentials(ctx, client, user, out, nil)
	}

	switch out.Format {
	case full:
		return printDatabaseBackups(backupList.Items, out, true)
	case noHeader:
		return printDatabaseBackups(backupList.Items, out, false)
	case yamlOut:
		return format.PrettyPrintObjects(backupList.GetItems(), format.PrintOpts{Out: &out.Writer})
	case jsonOut:
		return format.PrettyPrintObjects(
			backupList.GetItems(),
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

func printDatabaseBackups(backups []storage.DatabaseBackup, out *output, header bool) error {
	if header {
		out.writeHeader("NAME", "SOURCE", "ENGINE", "SIZE", "EXPIRES", "STATUS", "AGE")
	}

	for _, b := range backups {
		// the engine version is only known once the backup has run
		engine := b.Status.AtProvider.Version.Database
		if engine == "" {
			engine = noneText
		}

		out.writeTabRow(b.Namespace, b.Name,
			database.FormatRef(b.Spec.ForProvider.Source.Kind, b.Spec.ForProvider.Source.Name),
			engine,
			backupSize(b.Status.AtProvider.Size),
			expiresIn(b.Spec.ForProvider.Expiration.Time),
			string(b.Status.AtProvider.State),
			duration.HumanDuration(time.Since(b.CreationTimestamp.Time)),
		)
	}

	return out.tabWriter.Flush()
}

// backupSize formats the raw byte count rounded up to megabytes (e.g. "34M"),
// matching how database sizes are displayed.
func backupSize(size *resource.Quantity) string {
	if size == nil || size.Value() <= 0 {
		return noneText
	}
	rounded := resource.NewQuantity(size.Value(), resource.DecimalSI)
	rounded.RoundUp(resource.Mega)
	return rounded.String()
}

// expiresIn formats how long until the given time, or "<none>" if unset.
func expiresIn(t time.Time) string {
	if t.IsZero() {
		return noneText
	}
	return duration.HumanDuration(time.Until(t))
}

// backupBucketName returns the name of the bucket storing the backups of the
// given database, managed by its backup schedule.
func backupBucketName(ctx context.Context, client *api.Client, source meta.LocalTypedReference) (string, error) {
	schedules := &storage.DatabaseBackupScheduleList{}
	if err := client.List(ctx, schedules, runtimeclient.InNamespace(client.Project)); err != nil {
		return "", fmt.Errorf("unable to list backup schedules: %w", err)
	}

	for _, schedule := range schedules.Items {
		if !database.SameRef(schedule.Spec.ForProvider.Source, source) {
			continue
		}
		if schedule.Status.AtProvider.TargetBucket.Name == "" {
			return "", fmt.Errorf("the backup bucket of %s is not provisioned yet",
				database.FormatRef(source.Kind, source.Name))
		}
		return schedule.Status.AtProvider.TargetBucket.Name, nil
	}

	return "", fmt.Errorf("no backup schedule of %s found: make sure backups are enabled on the database",
		database.FormatRef(source.Kind, source.Name))
}

// backupBucketURL returns the URL of the named backup bucket and, if given,
// the path of a backup within it.
func backupBucketURL(ctx context.Context, client *api.Client, name, path string) (string, error) {
	bucket := &storage.Bucket{}
	if err := client.Get(ctx, client.Name(name), bucket); err != nil {
		return "", fmt.Errorf("unable to get backup bucket %q: %w", name, err)
	}

	elems := []string{bucket.Name}
	if path != "" {
		elems = append(elems, path)
	}
	return url.JoinPath(bucket.Status.AtProvider.Endpoint, elems...)
}

// backupBucketUser returns the BucketUser for read access to the named backup
// bucket. It is named after the bucket itself.
func backupBucketUser(ctx context.Context, client *api.Client, name string) (*storage.BucketUser, error) {
	user := &storage.BucketUser{}
	if err := client.Get(ctx, client.Name(name), user); err != nil {
		return nil, fmt.Errorf("unable to get bucket user of backup bucket %q: %w", name, err)
	}
	return user, nil
}
