package get

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/application"
	"github.com/ninech/nctl/internal/database"
	"github.com/ninech/nctl/internal/format"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/duration"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type databaseBackupCmd struct {
	ResourceCmd
	Source           application.TypedReference `placeholder:"kind/name" help:"Only show backups of this database, in the form kind/name (e.g. postgresdatabase/mydb)."`
	PrintURL         bool                       `help:"Print the URL of the backup in its bucket. Requires name to be set." xor:"print"`
	PrintCredentials bool                       `help:"Print credentials for read access to the backup's bucket. Requires name to be set." xor:"print"`
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
	if cmd.Source.Name != "" {
		source, err := database.LocalRef(cmd.Source.TypedReference)
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

	// the backup can be in another project than the client's, e.g. with --all-projects
	backup := backupList.Items[0]

	if cmd.Name != "" && cmd.PrintURL {
		if backup.Status.AtProvider.Path == "" {
			return fmt.Errorf("backup %q has no path yet, it might still be running", backup.Name)
		}
		url, err := backupBucketURL(ctx, client, backup.Namespace, backup.Spec.ForProvider.Bucket.Name, backup.Status.AtProvider.Path)
		if err != nil {
			return err
		}
		out.Println(url)
		return nil
	}

	if cmd.Name != "" && cmd.PrintCredentials {
		user, err := backupBucketUser(ctx, client, backup.Namespace, backup.Spec.ForProvider.Bucket.Name)
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
		out.writeTabRow(b.Namespace, b.Name,
			database.FormatRef(b.Spec.ForProvider.Source.Kind, b.Spec.ForProvider.Source.Name),
			// the engine version is only known once the backup has run
			noneIfEmpty(b.Status.AtProvider.Version.Database),
			backupSize(b.Status.AtProvider.Size),
			expiresIn(b.Status.AtProvider.Expiration.Time),
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
// HumanDuration cannot format negative durations.
func expiresIn(t time.Time) string {
	if t.IsZero() {
		return noneText
	}
	remaining := time.Until(t)
	if remaining <= 0 {
		return "expired"
	}
	return duration.HumanDuration(remaining)
}

func noneIfEmpty(s string) string {
	if s == "" {
		return noneText
	}
	return s
}

// backupBucketName returns the name of the bucket storing the backups of the
// given database in namespace, managed by its backup schedule.
func backupBucketName(ctx context.Context, client *api.Client, namespace string, source meta.LocalTypedReference) (string, error) {
	schedules := &storage.DatabaseBackupScheduleList{}
	if err := client.List(ctx, schedules, runtimeclient.InNamespace(namespace)); err != nil {
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
func backupBucketURL(ctx context.Context, client *api.Client, namespace, name, path string) (string, error) {
	bucket := &storage.Bucket{}
	if err := client.Get(ctx, api.NamespacedName(name, namespace), bucket); err != nil {
		return "", fmt.Errorf("unable to get backup bucket %q: %w", name, err)
	}
	endpoint := bucket.Status.AtProvider.Endpoint
	if endpoint == "" {
		return "", fmt.Errorf("the backup bucket %q is not provisioned yet", name)
	}
	if !strings.Contains(endpoint, "://") {
		endpoint = "https://" + endpoint
	}

	elems := []string{bucket.Name}
	if path != "" {
		elems = append(elems, path)
	}
	return url.JoinPath(endpoint, elems...)
}

// backupBucketUser returns the BucketUser for read access to the named backup
// bucket. It is named after the bucket itself.
func backupBucketUser(ctx context.Context, client *api.Client, namespace, name string) (*storage.BucketUser, error) {
	user := &storage.BucketUser{}
	if err := client.Get(ctx, api.NamespacedName(name, namespace), user); err != nil {
		return nil, fmt.Errorf("unable to get bucket user of backup bucket %q: %w", name, err)
	}
	return user, nil
}
