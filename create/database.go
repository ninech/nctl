package create

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/ninech/nctl/internal/database"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// createDatabase creates the database, reporting a clear error when the name is
// already taken.
func (c *creator) createDatabase(ctx context.Context) error {
	// check the name is free first, so a clash gives a clear error naming the
	// database
	existing := c.mg.DeepCopyObject().(runtimeclient.Object)
	if err := c.client.Get(ctx, runtimeclient.ObjectKeyFromObject(c.mg), existing); err == nil {
		return fmt.Errorf("%s %q already exists in project %q", c.kind, c.mg.GetName(), c.mg.GetNamespace())
	}
	return c.createResource(ctx)
}

// createFromRestore creates a database that restores an existing backup and
// waits for the composed restore to finish. restoreFromSet reports whether the
// restoreFrom reference survived a dry-run.
func (c *creator) createFromRestore(ctx context.Context, wait bool, backup string, restoreFromSet func(resource.Managed) bool) error {
	if err := c.ensureRestoreFromSupported(ctx, restoreFromSet); err != nil {
		return err
	}
	if err := c.createDatabase(ctx); err != nil {
		return err
	}
	if !wait {
		c.Infof("💡", "the restore continues on the server, check the progress with: nctl get databaserestores")
		return nil
	}
	stage, err := bootstrapWaitStage(c.kind, c.mg.GetName())
	if err != nil {
		return err
	}
	if err := c.wait(ctx, stage); err != nil {
		if errors.Is(err, database.ErrRestoreFailed) {
			return fmt.Errorf("%w\n\nthe %s %q is marked failed; delete it and create it again:\n nctl delete %s %s",
				err, c.kind, c.mg.GetName(), strings.ToLower(c.kind), c.mg.GetName())
		}
		return err
	}
	c.Successf("🚀", "%s %s restored from backup %q. You can retrieve the database, username and password with:\n\n nctl get %s %s --print-connection-string",
		c.kind, c.mg.GetName(), backup, strings.ToLower(c.kind), c.mg.GetName())
	return nil
}

// ensureRestoreFromSupported dry-runs the create and fails if the cluster does
// not accept the restoreFrom reference.
func (c *creator) ensureRestoreFromSupported(ctx context.Context, restoreFromSet func(resource.Managed) bool) error {
	dryRun := c.mg.DeepCopyObject().(resource.Managed)
	if err := c.client.Create(ctx, dryRun, runtimeclient.DryRunAll); err != nil {
		return fmt.Errorf("unable to validate the %s %q: %w", c.kind, c.mg.GetName(), err)
	}
	if !restoreFromSet(dryRun) {
		return fmt.Errorf("the cluster does not support restoring a %s from a backup yet: the restoreFrom field was rejected", c.kind)
	}
	return nil
}

// bootstrapWaitStage watches the freshly created database until its bootstrap
// restore finished. The database is watched instead of the restore, as a
// succeeded restore is garbage-collected and a watch on it can miss the
// outcome.
func bootstrapWaitStage(kind, name string) (waitStage, error) {
	list, err := database.ListFor(kind)
	if err != nil {
		return waitStage{}, err
	}
	return waitStage{
		objectList:  list,
		waitMessage: &message{icon: "⏳", text: "restoring backup"},
		doneMessage: &message{icon: "♻️", text: "backup restored"},
		onResult:    database.BootstrapCompleted(kind, name),
		timeoutHint: "the restore continues on the server, check the progress with: nctl get databaserestores",
	}, nil
}
