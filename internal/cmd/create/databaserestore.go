package create

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/application"
	"github.com/ninech/nctl/internal/database"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/waiter"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type databaseRestoreCmd struct {
	ResourceCmd
	format.Reader `kong:"-"`
	Backup        string                     `required:"" help:"Name of the DatabaseBackup to restore."`
	Target        application.TypedReference `required:"" placeholder:"kind/name" help:"Database to restore into, in the form kind/name (e.g. postgresdatabase/mydb)."`
	Force         bool                       `default:"false" help:"Do not ask for confirmation before replacing the data of the target."`
}

// BeforeApply initializes Writer and Reader from the bound [format.IOStreams].
func (cmd *databaseRestoreCmd) BeforeApply(streams format.IOStreams) error {
	return errors.Join(
		cmd.ResourceCmd.BeforeApply(streams),
		cmd.Reader.BeforeApply(streams),
	)
}

func (cmd *databaseRestoreCmd) Run(ctx context.Context, client *api.Client) error {
	target, err := database.LocalRef(cmd.Target.TypedReference)
	if err != nil {
		return err
	}
	targetRef := database.FormatRef(target.Kind, target.Name)

	// The admission accepts missing references, so a restore which cannot run
	// would wait for the full timeout.
	backup := &storage.DatabaseBackup{}
	if err := client.Get(ctx, client.Name(cmd.Backup), backup); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("backup %q does not exist", cmd.Backup)
		}
		return fmt.Errorf("unable to get backup %q: %w", cmd.Backup, err)
	}
	if err := restorable(backup); err != nil {
		return err
	}

	targetDB, err := database.New(target.Kind)
	if err != nil {
		return err
	}
	if err := client.Get(ctx, client.Name(target.Name), targetDB); err != nil {
		if apierrors.IsNotFound(err) {
			return fmt.Errorf("target %s does not exist", targetRef)
		}
		return fmt.Errorf("unable to get target %s: %w", targetRef, err)
	}
	if targetDB.GetDeletionTimestamp() != nil {
		return fmt.Errorf("target %s is being deleted", targetRef)
	}
	if err := noActiveRestore(ctx, client, target); err != nil {
		return err
	}

	restore := database.NewRestore(cmd.Name, client.Project, cmd.Backup, target)

	// The admission validates that the backup can be restored into the target,
	// e.g. its kind and version. A dry-run reports a rejection before the
	// confirmation.
	if err := client.Create(ctx, restore.DeepCopy(), runtimeclient.DryRunAll); err != nil {
		return fmt.Errorf("unable to restore backup %q into %s: %w", backup.Name, targetRef, err)
	}

	if !cmd.Force {
		cmd.Warningf("all data in %s is replaced with the data of backup %q from %s. Changes made after the backup are lost. The database is locked while the restore is running.",
			targetRef, backup.Name, backupTime(backup))
		ok, err := cmd.Confirm(cmd.Reader, fmt.Sprintf("Do you really want to restore backup %q into %s?", backup.Name, targetRef))
		if errors.Is(err, io.EOF) {
			return errors.New("no confirmation received, pass --force to restore without asking")
		}
		if err != nil {
			return err
		}
		if !ok {
			cmd.Failuref("", "%s canceled", storage.DatabaseRestoreKind)
			return nil
		}
		// Another restore may have started while the prompt was open.
		if err := noActiveRestore(ctx, client, target); err != nil {
			return err
		}
	}

	c := cmd.newCreator(client, restore, storage.DatabaseRestoreKind)
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !cmd.Wait {
		return nil
	}

	return c.wait(ctx, waiter.Stage{
		ObjectList:   &storage.DatabaseRestoreList{},
		WaitMessage:  &waiter.Message{Icon: "⏳", Text: "restoring database"},
		DoneMessage:  &waiter.Message{Icon: "♻️", Text: "database restored"},
		OnResult:     database.RestoreDone,
		ContinueHint: fmt.Sprintf("the restore continues on the server, check it with: nctl get databaserestores %s", restore.GetName()),
	})
}

// restorable returns an error if the backup has no data to restore. The
// provider only restores succeeded backups.
func restorable(backup *storage.DatabaseBackup) error {
	if state := backup.Status.AtProvider.State; state != storage.DatabaseJobStateSucceeded {
		return fmt.Errorf("backup %q is %s, only succeeded backups can be restored", backup.Name, state)
	}
	if backup.Status.AtProvider.Path == "" {
		return fmt.Errorf("backup %q has no data to restore", backup.Name)
	}
	return nil
}

// restoreStartGrace is how long a restore in the unknown state counts as
// active, until the provider sets its state.
const restoreStartGrace = 2 * time.Minute

// noActiveRestore returns an error if a restore into the named database is
// active. The provider would queue a new restore behind it.
func noActiveRestore(ctx context.Context, client *api.Client, target meta.LocalTypedReference) error {
	restores := &storage.DatabaseRestoreList{}
	if err := client.List(ctx, restores, runtimeclient.InNamespace(client.Project)); err != nil {
		return fmt.Errorf("unable to list restores: %w", err)
	}

	i := slices.IndexFunc(restores.Items, func(r storage.DatabaseRestore) bool {
		return database.SameRef(r.Spec.ForProvider.Target, target) &&
			r.DeletionTimestamp == nil &&
			restoreActive(&r)
	})
	if i < 0 {
		return nil
	}

	active := restores.Items[i].Name
	return fmt.Errorf("restore %q into %s is still in progress. Start a new restore once it has finished, check it with: nctl get databaserestores %s",
		active, database.FormatRef(target.Kind, target.Name), active)
}

// restoreActive reports whether the restore is pending, running or about to
// start.
func restoreActive(r *storage.DatabaseRestore) bool {
	switch r.Status.AtProvider.State {
	case storage.DatabaseJobStatePending, storage.DatabaseJobStateRunning:
		return true
	case storage.DatabaseJobStateUnknown:
		return time.Since(r.CreationTimestamp.Time) < restoreStartGrace
	}
	return false
}

func backupTime(backup *storage.DatabaseBackup) string {
	taken := backup.Status.AtProvider.Start.Time
	if taken.IsZero() {
		taken = backup.CreationTimestamp.Time
	}
	return taken.Local().Format(time.DateTime + " MST")
}
