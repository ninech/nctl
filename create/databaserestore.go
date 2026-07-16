package create

import (
	"context"
	"fmt"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/database"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/watch"
)

type databaseRestoreCmd struct {
	resourceCmd
	Backup string `required:"" help:"Name of the DatabaseBackup to restore."`
	Target string `required:"" help:"Database to restore into, in the form <kind>/<name> (e.g. postgresdatabase/mydb)."`
}

func (cmd *databaseRestoreCmd) Run(ctx context.Context, client *api.Client) error {
	target, err := database.ParseRef(cmd.Target)
	if err != nil {
		return err
	}

	// Fail fast if the target does not exist. The restore admission tolerates a
	// missing target so a restore can be applied alongside its database. A
	// restore into a target which never appears would wait for the full
	// timeout.
	targetDB, err := database.New(target.Kind)
	if err != nil {
		return err
	}
	if err := client.Get(ctx, client.Name(target.Name), targetDB); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("target %s does not exist", database.FormatRef(target.Kind, target.Name))
		}
		return fmt.Errorf("unable to get target %s: %w", database.FormatRef(target.Kind, target.Name), err)
	}

	restore := database.NewRestore(cmd.Name, client.Project, cmd.Backup, target)

	c := cmd.newCreator(client, restore, storage.DatabaseRestoreKind)
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !cmd.Wait {
		return nil
	}

	return c.wait(ctx, waitStage{
		Writer:      cmd.Writer,
		objectList:  &storage.DatabaseRestoreList{},
		waitMessage: &message{icon: "⏳", text: "restoring database"},
		doneMessage: &message{icon: "♻️", text: "database restored"},
		onResult:    restoreDoneNamed(restore.GetName()),
		timeoutHint: fmt.Sprintf("the restore continues on the server, check it with: nctl get databaserestores %s", restore.GetName()),
	})
}

// restoreDoneNamed reports whether the named restore has finished, ignoring
// other restores in the project (e.g. a stale one the watch replays on start).
func restoreDoneNamed(name string) func(watch.Event) (bool, error) {
	return func(event watch.Event) (bool, error) {
		restore, ok := event.Object.(*storage.DatabaseRestore)
		if !ok || restore.Name != name {
			return false, nil
		}
		return database.RestoreDone(event)
	}
}
