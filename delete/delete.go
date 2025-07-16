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
	Filename            string               `short:"f" predictor:"file"`
	FromFile            fromFile             `cmd:"" default:"1" name:"-f <file>" help:"Delete any resource from a yaml or json file."`
	VCluster            vclusterCmd          `cmd:"" group:"infrastructure.nine.ch" name:"vcluster" help:"Delete a vcluster."`
	APIServiceAccount   apiServiceAccountCmd `cmd:"" group:"iam.nine.ch" name:"apiserviceaccount" aliases:"asa" help:"Delete an API Service Account."`
	Project             projectCmd           `cmd:"" group:"management.nine.ch" name:"project" aliases:"proj" help:"Delete a Project."`
	Config              configCmd            `cmd:"" group:"deplo.io" name:"config" help:"Delete a deplo.io Project Configuration."`
	Application         applicationCmd       `cmd:"" group:"deplo.io" name:"application" aliases:"app,application" help:"Delete a deplo.io Application."`
	MySQL               mySQLCmd             `cmd:"" group:"storage.nine.ch" name:"mysql" help:"Delete a MySQL instance."`
	MySQLDatabase       mysqlDatabaseCmd     `cmd:"" group:"storage.nine.ch" name:"mysqldatabase" help:"Delete a MySQL database."`
	Postgres            postgresCmd          `cmd:"" group:"storage.nine.ch" name:"postgres" help:"Delete a PostgreSQL instance."`
	PostgresDatabase    postgresDatabaseCmd  `cmd:"" group:"storage.nine.ch" name:"postgresdatabase" help:"Delete a PostgreSQL database."`
	KeyValueStore       keyValueStoreCmd     `cmd:"" group:"storage.nine.ch" name:"keyvaluestore" aliases:"kvs" help:"Delete a KeyValueStore instance."`
	CloudVirtualMachine cloudVMCmd           `cmd:"" group:"infrastructure.nine.ch" name:"cloudvirtualmachine" aliases:"cloudvm" help:"Delete a CloudVM."`
	ServiceConnection   serviceConnectionCmd `cmd:"" group:"networking.nine.ch" name:"serviceconnection" aliases:"sc" help:"Delete a ServiceConnection."`
}

type resourceCmd struct {
	Name        string        `arg:"" predictor:"resource_name" help:"Name of the resource to delete."`
	Force       bool          `default:"false" help:"Do not ask for confirmation of deletion."`
	Wait        bool          `default:"true" help:"Wait until resource is fully deleted"`
	WaitTimeout time.Duration `default:"5m" help:"Duration to wait for the deletion. Only relevant if wait is set."`
}

// cleanupFunc is called after the resource has been deleted in order to do
// any sort of cleanups.
type cleanupFunc func(client *api.Client) error

// promptFunc can be used to create a special prompt when asking for deletion
type promptFunc func(kind, name string) string

type deleter struct {
	kind    string
	mg      resource.Managed
	cleanup cleanupFunc
	prompt  promptFunc
}

// deleterOption allows to set options for the deletion
type deleterOption func(*deleter)

func newDeleter(mg resource.Managed, kind string, opts ...deleterOption) *deleter {
	d := &deleter{
		kind:    kind,
		mg:      mg,
		cleanup: noCleanup,
		prompt:  defaultPrompt,
	}
	for _, opt := range opts {
		opt(d)
	}

	return d
}

// cleanup allows to set a cleanup function
func cleanup(cleanup cleanupFunc) deleterOption {
	return func(d *deleter) {
		d.cleanup = cleanup
	}
}

// prompt allows to alter the deletion prompt
func prompt(prompt promptFunc) deleterOption {
	return func(d *deleter) {
		d.prompt = prompt
	}
}

func noCleanup(client *api.Client) error {
	return nil
}

func defaultPrompt(kind, name string) string {
	return fmt.Sprintf("do you really want to delete the %s %q?", kind, name)
}

func (d *deleter) deleteResource(ctx context.Context, client *api.Client, waitTimeout time.Duration, wait, force bool) error {
	ctx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()

	// check if the the resource even exists before going any further
	if err := client.Get(ctx, client.Name(d.mg.GetName()), d.mg); err != nil {
		return fmt.Errorf("unable to get %s %q: %w", d.kind, d.mg.GetName(), err)
	}

	if !force {
		ok, err := format.Confirm(d.prompt(d.kind, d.mg.GetName()))
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
			switch ctx.Err() {
			case context.DeadlineExceeded:
				msg := "timeout waiting for %s"
				spinner.StopFailMessage(format.ProgressMessagef("", msg, d.kind))
				_ = spinner.StopFail()

				return fmt.Errorf(msg, d.kind)
			case context.Canceled:
				_ = spinner.StopFail()
				return nil
			}
		}
	}
}
