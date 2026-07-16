package get

import (
	"context"
	"fmt"
	"slices"
	"time"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/database"
	"github.com/ninech/nctl/internal/format"
	"k8s.io/apimachinery/pkg/util/duration"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type databaseRestoreCmd struct {
	resourceCmd
	Target string `help:"Only show restores into this database, in the form <kind>/<name> (e.g. postgresdatabase/mydb)."`
}

func (cmd *databaseRestoreCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	return get.listPrint(ctx, client, cmd, api.MatchName(cmd.Name))
}

func (cmd *databaseRestoreCmd) list() runtimeclient.ObjectList {
	return &storage.DatabaseRestoreList{}
}

func (cmd *databaseRestoreCmd) print(ctx context.Context, client *api.Client, list runtimeclient.ObjectList, out *output) error {
	restoreList, ok := list.(*storage.DatabaseRestoreList)
	if !ok {
		return fmt.Errorf("expected %T, got %T", &storage.DatabaseRestoreList{}, list)
	}
	if cmd.Target != "" {
		target, err := database.ParseRef(cmd.Target)
		if err != nil {
			return err
		}
		restoreList.Items = slices.DeleteFunc(restoreList.Items, func(r storage.DatabaseRestore) bool {
			return !database.SameRef(r.Spec.ForProvider.Target, target)
		})
	}
	if len(restoreList.Items) == 0 {
		return out.notFound(storage.DatabaseRestoreKind, client.Project)
	}

	switch out.Format {
	case full:
		return printDatabaseRestores(restoreList.Items, out, true)
	case noHeader:
		return printDatabaseRestores(restoreList.Items, out, false)
	case yamlOut:
		return format.PrettyPrintObjects(restoreList.GetItems(), format.PrintOpts{Out: &out.Writer})
	case jsonOut:
		return format.PrettyPrintObjects(
			restoreList.GetItems(),
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

func printDatabaseRestores(restores []storage.DatabaseRestore, out *output, header bool) error {
	if header {
		out.writeHeader("NAME", "BACKUP", "TARGET", "STATUS", "AGE")
	}

	for _, r := range restores {
		out.writeTabRow(r.Namespace, r.Name,
			r.Spec.ForProvider.Backup.Name,
			database.FormatRef(r.Spec.ForProvider.Target.Kind, r.Spec.ForProvider.Target.Name),
			string(r.Status.AtProvider.State),
			duration.HumanDuration(time.Since(r.CreationTimestamp.Time)),
		)
	}

	return out.tabWriter.Flush()
}
