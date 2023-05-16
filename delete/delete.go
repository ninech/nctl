package delete

import (
	"context"
	"fmt"
	"time"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"k8s.io/apimachinery/pkg/api/errors"
)

type Cmd struct {
	Filename          string               `short:"f" predictor:"file"`
	FromFile          fromFile             `cmd:"" default:"1" name:"-f <file>" help:"Delete any resource from a yaml or json file."`
	VCluster          vclusterCmd          `cmd:"" name:"vcluster" help:"Delete a vcluster."`
	APIServiceAccount apiServiceAccountCmd `cmd:"" name:"apiserviceaccount" aliases:"asa" help:"Delete an API Service Account."`
	Application       applicationCmd       `cmd:"" name:"application" aliases:"app" help:"Delete a deplo.io Application. (Beta - requires access)"`
}

// cleanupFunc is called after the resource has been deleted in order to do
// any sort of cleanups.
type cleanupFunc func(client *api.Client) error

type deleter struct {
	kind    string
	mg      resource.Managed
	cleanup cleanupFunc
}

func newDeleter(mg resource.Managed, kind string, cleanup cleanupFunc) *deleter {
	return &deleter{
		kind:    kind,
		mg:      mg,
		cleanup: cleanup,
	}
}

func noCleanup(client *api.Client) error {
	return nil
}

func (d *deleter) deleteResource(ctx context.Context, client *api.Client, waitTimeout time.Duration, wait, force bool) error {
	ctx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()

	// check if the the resource even exists before going any further
	if err := client.Get(ctx, client.Name(d.mg.GetName()), d.mg); err != nil {
		return fmt.Errorf("unable to get %s %q: %w", d.kind, d.mg.GetName(), err)
	}

	if !force {
		ok, err := format.Confirmf("do you really want to delete the %s %q?", d.kind, d.mg.GetName())
		if err != nil {
			return err
		}
		if !ok {
			format.PrintFailuref("", "%s deletion canceled", d.kind)
			return nil
		}
	}

	if err := client.Delete(ctx, d.mg); err != nil {
		return fmt.Errorf("unable to delete %s %q: %w", d.kind, d.mg.GetName(), err)
	}

	if wait {
		if err := d.waitForDeletion(ctx, client); err != nil {
			return err
		}
	} else {
		format.PrintSuccessf("🗑", "%s deletion started", d.kind)
	}

	return d.cleanup(client)
}

func (d *deleter) waitForDeletion(ctx context.Context, client *api.Client) error {
	spinner, err := format.NewSpinner(
		format.ProgressMessagef("⏳", "%s is being deleted", d.kind),
		format.ProgressMessagef("🗑", "%s deleted", d.kind),
	)
	if err != nil {
		return err
	}

	_ = spinner.Start()
	defer func() { _ = spinner.Stop() }()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := client.Get(ctx, client.Name(d.mg.GetName()), d.mg); err != nil {
				if errors.IsNotFound(err) {
					_ = spinner.Stop()
					return nil
				}

				_ = spinner.StopFail()
				return fmt.Errorf("unable to get %s %q: %w", d.kind, d.mg.GetName(), err)
			}
		case <-ctx.Done():
			msg := "timeout waiting for %s"
			spinner.StopFailMessage(format.ProgressMessagef("", msg, d.kind))
			_ = spinner.StopFail()

			return fmt.Errorf("timeout waiting for %s", d.kind)
		}
	}
}
