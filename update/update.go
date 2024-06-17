package update

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type Cmd struct {
	Application         applicationCmd   `cmd:"" group:"deplo.io" name:"application" aliases:"app" help:"Update an existing deplo.io Application. (Beta - requires access)"`
	Config              configCmd        `cmd:"" group:"deplo.io" name:"config"  help:"Update an existing deplo.io Project Configuration. (Beta - requires access)"`
	Project             projectCmd       `cmd:"" group:"management.nine.ch" name:"project"  help:"Update an existing Project"`
	MySQL               mySQLCmd         `cmd:"" group:"storage.nine.ch" name:"mysql" help:"Update an existing MySQL instance."`
	Postgres            postgresCmd      `cmd:"" group:"storage.nine.ch" name:"postgres" help:"Update an existing PostgreSQL instance."`
	KeyValueStore       keyValueStoreCmd `cmd:"" group:"storage.nine.ch" name:"keyvaluestore" aliases:"kvs" help:"Update an existing KeyValueStore instance"`
	CloudVirtualMachine cloudVMCmd       `cmd:"" group:"infrastructure.nine.ch" name:"cloudvirtualmachine" aliases:"cloudvm" help:"Update a CloudVM."`
}

type resourceCmd struct {
	Name string `arg:"" predictor:"resource_name" help:"Name of the resource to update."`
}

type updater struct {
	mg         resource.Managed
	client     *api.Client
	kind       string
	updateFunc updateFunc
}

type updateFunc func(current resource.Managed) error

func newUpdater(client *api.Client, mg resource.Managed, kind string, f updateFunc) *updater {
	return &updater{client: client, mg: mg, kind: kind, updateFunc: f}
}

func (u *updater) Update(ctx context.Context) error {
	if err := u.client.Get(ctx, api.ObjectName(u.mg), u.mg); err != nil {
		return err
	}

	if err := u.updateFunc(u.mg); err != nil {
		return err
	}

	if err := u.client.Update(ctx, u.mg); err != nil {
		return err
	}

	format.PrintSuccessf("⬆️", "updated %s %q", u.kind, u.mg.GetName())
	return nil
}
