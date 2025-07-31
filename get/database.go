package get

import (
	"context"
	"fmt"

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
}

func (cmd *databaseCmd) run(ctx context.Context, client *api.Client, get *Cmd,
	databaseResources resource.ManagedList, databaseKind string,
	connectionString func(resource.Managed, map[string][]byte) (string, error),
	printList func(resource.ManagedList, *Cmd, bool) error,
	caCert func(resource.Managed) (string, error),
) error {
	if len(databaseResources.GetItems()) == 0 {
		return get.printEmptyMessage(databaseKind, client.Project)
	}

	if cmd.Name != "" && cmd.PrintUser {
		return cmd.printSecret(get.writer, ctx, client, databaseResources.GetItems()[0], func(db, _ string) string { return db })
	}

	if cmd.Name != "" && cmd.PrintPassword {
		return cmd.printSecret(get.writer, ctx, client, databaseResources.GetItems()[0], func(_, pw string) string { return pw })
	}

	if cmd.Name != "" && cmd.PrintConnectionString {
		secrets, err := getConnectionSecretMap(ctx, client, databaseResources.GetItems()[0])
		if err != nil {
			return err
		}

		str, err := connectionString(databaseResources.GetItems()[0], secrets)
		if err != nil {
			return err
		}

		_, err = fmt.Fprintln(get.writer, str)
		return err
	}

	if cmd.Name != "" && cmd.PrintCACert {
		ca, err := caCert(databaseResources.GetItems()[0])
		if err != nil {
			return err
		}
		return printBase64(get.writer, ca)
	}

	switch get.Format {
	case full:
		return printList(databaseResources, get, true)
	case noHeader:
		return printList(databaseResources, get, false)
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
