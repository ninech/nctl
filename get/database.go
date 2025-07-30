package get

import (
	"context"
	"io"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type databaseCmd struct {
	resourceCmd
	PrintPassword         bool `help:"Print the password of the database. Requires name to be set." xor:"print"`
	PrintUser             bool `help:"Print the database name and user of the database. Requires name to be set." xor:"print" aliases:"print-database-user"`
	PrintConnectionString bool `help:"Print the connection string of the database. Requires name to be set." xor:"print"`
	PrintCACert           bool `help:"Print the ca certificate. Requires name to be set." xor:"print"`

	out io.Writer
}

func (cmd *databaseCmd) run(ctx context.Context, client *api.Client, get *Cmd,
	databaseResources resource.ManagedList, databaseKind string,
	connectionStringHandler func(context.Context, *api.Client, resource.Managed) error,
	databasesHandler func([]resource.Managed, *Cmd, bool) error,
	caCertHandler func(resource.Managed) (string, error),
) error {
	if cmd.out == nil {
		cmd.out = get.writer
	}

	if len(databaseResources.GetItems()) == 0 {
		return get.printEmptyMessage(databaseKind, client.Project)
	}

	if cmd.Name != "" && cmd.PrintConnectionString {
		return connectionStringHandler(ctx, client, databaseResources.GetItems()[0])
	}

	if cmd.Name != "" && cmd.PrintUser {
		return cmd.printSecret(cmd.out, ctx, client, databaseResources.GetItems()[0], func(db, _ string) string { return db })
	}

	if cmd.Name != "" && cmd.PrintPassword {
		return cmd.printSecret(cmd.out, ctx, client, databaseResources.GetItems()[0], func(_, pw string) string { return pw })
	}
	if cmd.Name != "" && cmd.PrintCACert {
		ca, err := caCertHandler(databaseResources.GetItems()[0])
		if err != nil {
			return err
		}
		return printBase64(cmd.out, ca)
	}
	if cmd.Name != "" && cmd.PrintCACert {
		ca, err := caCertHandler(databaseResources.GetItems()[0])
		if err != nil {
			return err
		}
		return printBase64(cmd.out, ca)
	}

	switch get.Format {
	case full:
		return databasesHandler(databaseResources.GetItems(), get, true)
	case noHeader:
		return databasesHandler(databaseResources.GetItems(), get, false)
	case yamlOut:
		return format.PrettyPrintObjects(databaseResources.GetItems(), format.PrintOpts{Out: get.writer})
	case jsonOut:
		return format.PrettyPrintObjects(
			databaseResources.GetItems(),
			format.PrintOpts{
				Out:    get.writer,
				Format: format.OutputFormatTypeJSON,
				JSONOpts: format.JSONOutputOptions{
					PrintSingleItem: cmd.Name != "",
				},
			})
	}

	return nil
}
