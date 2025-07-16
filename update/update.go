package update

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type Cmd struct {
	Application         applicationCmd       `cmd:"" group:"deplo.io" name:"application" aliases:"app,application" help:"Update an existing deplo.io Application."`
	Config              configCmd            `cmd:"" group:"deplo.io" name:"config"  help:"Update an existing deplo.io Project Configuration."`
	Project             projectCmd           `cmd:"" group:"management.nine.ch" name:"project"  help:"Update an existing Project."`
	MySQL               mySQLCmd             `cmd:"" group:"storage.nine.ch" name:"mysql" help:"Update an existing MySQL instance."`
	MySQLDatabase       mysqlDatabaseCmd     `cmd:"" group:"storage.nine.ch" name:"mysqldatabase" help:"Update an existing MySQL database."`
	Postgres            postgresCmd          `cmd:"" group:"storage.nine.ch" name:"postgres" help:"Update an existing PostgreSQL instance."`
	PostgresDatabase    postgresDatabaseCmd  `cmd:"" group:"storage.nine.ch" name:"postgresdatabase" help:"Update an existing PostgreSQL database."`
	KeyValueStore       keyValueStoreCmd     `cmd:"" group:"storage.nine.ch" name:"keyvaluestore" aliases:"kvs" help:"Update an existing KeyValueStore instance."`
	CloudVirtualMachine cloudVMCmd           `cmd:"" group:"infrastructure.nine.ch" name:"cloudvirtualmachine" aliases:"cloudvm" help:"Update a CloudVM."`
	ServiceConnection   serviceConnectionCmd `cmd:"" group:"networking.nine.ch" name:"serviceconnection" aliases:"sc" help:"Update a ServiceConnection."`
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
