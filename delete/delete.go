package delete

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/ninech/nctl/api"
	"k8s.io/apimachinery/pkg/api/errors"
)

type Cmd struct {
	VCluster          vclusterCmd          `cmd:"" name:"vcluster" help:"Delete a vcluster."`
	APIServiceAccount apiServiceAccountCmd `cmd:"" name:"apiserviceaccount" aliases:"asa" help:"Delete a new API Service Account."`
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
		if !confirmf("do you really want to delete the %s %q?", d.kind, d.mg.GetName()) {
			fmt.Printf(" ✗ %s deletion canceled\n", d.kind)
			return nil
		}
	}

	if err := client.Delete(ctx, d.mg); err != nil {
		return fmt.Errorf("unable to delete %s %q: %w", d.kind, d.mg.GetName(), err)
	}
	fmt.Printf(" ✓ %s deletion started\n", d.kind)

	if wait {
		if err := d.waitForDeletion(ctx, client); err != nil {
			return err
		}
	}

	return d.cleanup(client)
}

func (d *deleter) waitForDeletion(ctx context.Context, client *api.Client) error {
	spin := spinner.New(spinner.CharSets[7], 100*time.Millisecond)
	spin.Prefix = " "
	spin.Start()
	defer spin.Stop()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := client.Get(ctx, client.Name(d.mg.GetName()), d.mg); err != nil {
				if errors.IsNotFound(err) {
					spin.Stop()

					fmt.Printf(" ✓ %s deleted 🗑\n", d.kind)
					return nil
				}
				return fmt.Errorf("unable to get %s %q: %w", d.kind, d.mg.GetName(), err)
			}
		case <-ctx.Done():
			spin.Stop()
			return fmt.Errorf("timeout waiting for %s", d.kind)
		}
	}
}

func confirmf(format string, a ...any) bool {
	var input string

	fmt.Printf("%s [y|n]: ", fmt.Sprintf(format, a...))
	_, err := fmt.Scanln(&input)
	if err != nil {
		panic(err)
	}
	input = strings.ToLower(input)

	if input == "y" || input == "yes" {
		return true
	}
	return false
}
