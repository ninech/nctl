// Package update contains the commands for updating resources.
package update

import (
	"context"
	"io"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type Cmd struct {
	Application         applicationCmd       `cmd:"" group:"update-apps" name:"application" aliases:"app,application" help:"Update an existing deplo.io Application."`
	APIServiceAccount   apiServiceAccountCmd `cmd:"" group:"update-access" name:"apiserviceaccount" aliases:"asa" help:"Update an API Service Account."`
	ProjectConfig       configCmd            `cmd:"" group:"update-apps" name:"project-config" aliases:"config" help:"Update an existing deplo.io Project Configuration."`
	Project             projectCmd           `cmd:"" group:"update-access" name:"project"  help:"Update an existing Project."`
	MySQL               mySQLCmd             `cmd:"" group:"update-storage" name:"mysql" help:"Update an existing MySQL instance."`
	MySQLDatabase       mysqlDatabaseCmd     `cmd:"" group:"update-storage" name:"mysqldatabase" help:"Update an existing MySQL database."`
	Postgres            postgresCmd          `cmd:"" group:"update-storage" name:"postgres" help:"Update an existing PostgreSQL instance."`
	PostgresDatabase    postgresDatabaseCmd  `cmd:"" group:"update-storage" name:"postgresdatabase" help:"Update an existing PostgreSQL database."`
	KeyValueStore       keyValueStoreCmd     `cmd:"" group:"update-storage" name:"keyvaluestore" aliases:"kvs" help:"Update an existing KeyValueStore instance."`
	OpenSearch          openSearchCmd        `cmd:"" group:"update-storage" name:"opensearch" aliases:"os" help:"Update an existing OpenSearch cluster."`
	CloudVirtualMachine cloudVMCmd           `cmd:"" group:"update-infra" name:"cloudvirtualmachine" aliases:"cloudvm" help:"Update a CloudVM."`
	ServiceConnection   serviceConnectionCmd `cmd:"" group:"update-network" name:"serviceconnection" aliases:"sc" help:"Update a ServiceConnection."`
	BucketUser          bucketUserCmd        `cmd:"" group:"update-storage" name:"bucketuser" aliases:"bu" help:"Update a BucketUser."`
	Bucket              bucketCmd            `cmd:"" group:"update-storage" name:"bucket" help:"Update a Bucket."`
	Grafana             grafanaCmd           `cmd:"" group:"update-observability" name:"grafana" help:"Update an existing Grafana instance."`
}

type resourceCmd struct {
	format.Writer `kong:"-"`
	Name          string `arg:"" completion-predictor:"resource_name" help:"Name of the resource to update."`
}

// BeforeApply initializes Writer from Kong's bound [io.Writer].
func (cmd *resourceCmd) BeforeApply(writer io.Writer) error {
	return cmd.Writer.BeforeApply(writer)
}

type updater struct {
	format.Writer
	mg         resource.Managed
	client     *api.Client
	kind       string
	updateFunc updateFunc
}

type updateFunc func(current resource.Managed) error

func (cmd *resourceCmd) newUpdater(
	client *api.Client,
	mg resource.Managed,
	kind string,
	f updateFunc,
) *updater {
	return &updater{Writer: cmd.Writer, client: client, mg: mg, kind: kind, updateFunc: f}
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

	u.Successf("⬆️", "updated %s %q", u.kind, u.mg.GetName())
	return nil
}
