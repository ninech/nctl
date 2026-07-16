package copy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/database"
	"github.com/ninech/nctl/internal/format"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// databaseCmd contains the common flags of the database copy commands. A copy
// always clones a fresh backup of the source. To create a database from an
// existing backup use 'nctl create <kind> --restore-from'.
type databaseCmd struct {
	format.Writer `kong:"-"`
	Name          string        `arg:"" completion-predictor:"resource_name" help:"Name of the source database to copy."`
	TargetName    string        `help:"Name of the new database. A random name is generated if omitted." default:""`
	TargetVersion string        `help:"Version of the new database, e.g. to upgrade to a newer release. Defaults to the source version." default:""`
	WaitTimeout   time.Duration `default:"30m" help:"Duration to wait for the copy to complete."`
	Wait          bool          `default:"true" negatable:"" help:"Wait for the copy to complete."`
}

// BeforeApply initializes Writer from Kong's bound [io.Writer].
func (cmd *databaseCmd) BeforeApply(writer io.Writer) error {
	return cmd.Writer.BeforeApply(writer)
}

// copyContinuesHint is shown when the copy continues asynchronously on the server.
const copyContinuesHint = "the copy continues on the server, check the progress with: nctl get databaserestores"

// databaseCopy describes the kind-specific parts of a database copy.
type databaseCopy struct {
	// kind of the database being copied.
	kind string
	// newDB is the new database to be created, carrying cloneFrom (a fresh
	// backup of the source).
	newDB resource.Managed
	// cloneFromSet reports whether the new database still carries its cloneFrom
	// reference after a dry-run.
	cloneFromSet func(resource.Managed) bool
}

// run copies a database by creating the new database with cloneFrom (a fresh
// backup of the source) set, then waiting for the composed restore to finish.
func (cmd *databaseCmd) run(ctx context.Context, client *api.Client, dc databaseCopy) error {
	// check the target name is free first, so a clash gives a clear error
	// naming the database
	existing := dc.newDB.DeepCopyObject().(resource.Managed)
	if err := client.Get(ctx, runtimeclient.ObjectKeyFromObject(dc.newDB), existing); err == nil {
		return fmt.Errorf("%s %q already exists in project %q", dc.kind, dc.newDB.GetName(), dc.newDB.GetNamespace())
	}

	// a cluster whose CRD does not know cloneFrom drops it silently, so a
	// dry-run confirms it is accepted before anything is created
	dryRun := dc.newDB.DeepCopyObject().(resource.Managed)
	if err := client.Create(ctx, dryRun, runtimeclient.DryRunAll); err != nil {
		return fmt.Errorf("unable to validate the copy of %s %q: %w", dc.kind, dc.newDB.GetName(), err)
	}
	if !dc.cloneFromSet(dryRun) {
		return fmt.Errorf("the cluster does not support copying a %s yet: the cloneFrom field was rejected", dc.kind)
	}

	cmd.Infof("💾", "%s", cloneStartMessage(dc.kind, cmd.Name))

	if err := client.Create(ctx, dc.newDB); err != nil {
		return fmt.Errorf("unable to create %s %q: %w", dc.kind, dc.newDB.GetName(), err)
	}
	cmd.Successf("🏗", "created %s %q in project %q", dc.kind, dc.newDB.GetName(), dc.newDB.GetNamespace())

	if !cmd.Wait {
		cmd.Infof("💡", copyContinuesHint)
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	start := time.Now()
	completed, err := cmd.waitFor(ctx, client, dc,
		database.BootstrapCompleted(dc.kind, dc.newDB.GetName()))
	if err != nil {
		if errors.Is(err, database.ErrRestoreFailed) {
			return fmt.Errorf("%w\n\nthe %s %q is marked failed; delete it and run the copy again:\n nctl delete %s %s",
				err, dc.kind, dc.newDB.GetName(), strings.ToLower(dc.kind), dc.newDB.GetName())
		}
		return err
	}
	if !completed {
		// waitFor already printed the copy continues hint
		return nil
	}

	cmd.Successf("🚀", "%s %q copied to %q (%s). You can retrieve the database, username and password with:\n\n nctl get %s %s --print-connection-string",
		dc.kind, cmd.Name, dc.newDB.GetName(), time.Since(start).Truncate(time.Second), strings.ToLower(dc.kind), dc.newDB.GetName())

	return nil
}

// cloneStartMessage describes a copy that takes a fresh backup of the source.
func cloneStartMessage(kind, source string) string {
	return fmt.Sprintf("copying from a fresh backup of %s %q", strings.ToLower(kind), source)
}

// watchBackoff shapes the reconnect delays after a dropped watch. The drop
// budget is refreshed whenever a watch delivered an event before dropping, so
// a long copy survives routine watch recycling.
// nolint:gochecknoglobals
var watchBackoff = wait.Backoff{
	Steps:    8,
	Duration: 250 * time.Millisecond,
	Factor:   2.0,
	Jitter:   0.1,
	Cap:      15 * time.Second,
}

// errWatchDropped reports a watch that ended without a result, e.g. because
// the API server recycled the connection. It is retried with watchBackoff.
var errWatchDropped = errors.New("restore watch dropped")

// waitFor watches the new database until done reports its bootstrap restore
// finished, reconnecting with backoff when the watch drops. It returns false
// without an error when the wait was interrupted, in which case the copy
// continues on the server.
func (cmd *databaseCmd) waitFor(
	ctx context.Context,
	client *api.Client,
	dc databaseCopy,
	done func(watch.Event) (bool, error),
) (bool, error) {
	message := fmt.Sprintf("copying into %s %q", dc.kind, dc.newDB.GetName())
	progress := func() string { return format.ProgressWithRemaining(ctx, "⏳", message) }
	spinner, err := cmd.Spinner(progress(), format.Progress("⏳", message))
	if err != nil {
		return false, err
	}
	_ = spinner.Start()

	backoff := watchBackoff
	drops := 0
	for {
		finished, progressed, err := cmd.watchTarget(ctx, client, dc, done,
			func() { spinner.Message(progress()) })
		switch {
		case err == nil && finished:
			_ = spinner.Stop()
			return true, nil
		case err == nil:
			// the wait was interrupted and the copy continues on the server
			_ = spinner.StopFail()
			cmd.Infof("💡", copyContinuesHint)
			return false, nil
		case errors.Is(err, errWatchDropped):
			if progressed {
				// the last watch worked, so refresh the drop budget
				backoff = watchBackoff
				drops = 0
			}
			if drops++; drops >= watchBackoff.Steps {
				_ = spinner.StopFail()
				return false, fmt.Errorf("error watching restores, the API might be experiencing connectivity issues; %s", copyContinuesHint)
			}
			select {
			case <-time.After(backoff.Step()):
			case <-ctx.Done():
				// the next watch attempt reports how the wait ended
			}
		default:
			_ = spinner.StopFail()
			return false, err
		}
	}
}

// watchTarget runs a single watch on the new database until done reports the
// bootstrap restore finished, the context ends, or the watch drops
// (errWatchDropped). progressed reports whether the watch delivered at least
// one event before it ended.
func (cmd *databaseCmd) watchTarget(
	ctx context.Context,
	client *api.Client,
	dc databaseCopy,
	done func(watch.Event) (bool, error),
	updateProgress func(),
) (finished, progressed bool, err error) {
	if ctx.Err() != nil {
		return false, false, waitEnded(ctx, client, dc)
	}

	list, err := database.ListFor(dc.kind)
	if err != nil {
		return false, false, err
	}
	watcher, err := client.Watch(ctx, list,
		runtimeclient.InNamespace(client.Project),
		runtimeclient.MatchingFields{"metadata.name": dc.newDB.GetName()},
	)
	if err != nil {
		if ctx.Err() != nil {
			return false, false, waitEnded(ctx, client, dc)
		}
		return false, false, fmt.Errorf("%w: %v", errWatchDropped, err)
	}
	defer watcher.Stop()

	// The bootstrap restore may have finished before this watch started. Get
	// the database and check its status. The watch is established first so
	// nothing in between is missed. Seeing the outcome twice is harmless.
	target := dc.newDB.DeepCopyObject().(resource.Managed)
	if err := client.Get(ctx, runtimeclient.ObjectKeyFromObject(target), target); err == nil {
		finished, err := done(watch.Event{Type: watch.Modified, Object: target})
		if err != nil {
			return false, progressed, err
		}
		if finished {
			return true, progressed, nil
		}
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok || event.Type == watch.Error {
				return false, progressed, errWatchDropped
			}
			progressed = true

			finished, err := done(event)
			if err != nil {
				return false, progressed, err
			}
			if finished {
				return true, progressed, nil
			}
		case <-ticker.C:
			updateProgress()
		case <-ctx.Done():
			return false, progressed, waitEnded(ctx, client, dc)
		}
	}
}

// waitEnded translates an ended wait context into the result. An interrupt is
// not an error, as the copy continues on the server. A timeout is an error
// and includes where the copy currently stands.
func waitEnded(ctx context.Context, client *api.Client, dc databaseCopy) error {
	if errors.Is(ctx.Err(), context.Canceled) {
		return nil
	}
	return fmt.Errorf("timed out copying into %s %q; %s%s",
		dc.kind, dc.newDB.GetName(), copyContinuesHint, targetConditionHint(ctx, client, dc))
}

// targetConditionHint describes why the target database is not progressing,
// e.g. when the copy stalled before its restore was even composed. It is best
// effort and an unreachable database yields no hint.
func targetConditionHint(ctx context.Context, client *api.Client, dc databaseCopy) string {
	getCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	target := dc.newDB.DeepCopyObject().(resource.Managed)
	if err := client.Get(getCtx, runtimeclient.ObjectKeyFromObject(target), target); err != nil {
		return ""
	}
	synced := target.GetCondition(runtimev1.TypeSynced)
	if synced.Status == corev1.ConditionFalse && synced.Message != "" {
		return fmt.Sprintf("; the %s reports: %s", strings.ToLower(dc.kind), synced.Message)
	}
	return ""
}
