package get

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type Database struct {
	Namespace, Name, FQDN, Location, Size, Connections string
}

type databaseCmd struct {
	resourceCmd
	PrintPassword         bool `help:"Print the password of the database. Requires name to be set." xor:"print"`
	PrintDatabaseUser     bool `help:"Print the database name and user of the database. Requires name to be set." xor:"print"`
	PrintConnectionString bool `help:"Print the connection string of the database. Requires name to be set." xor:"print"`
}

func (cmd *databaseCmd) runDatabaseCmd(ctx context.Context, client *api.Client,
	out *output, databaseResources []resource.Managed, databaseKind string,
	connectionStringHandler func(context.Context, *api.Client, resource.Managed) error,
	databasesHandler func([]resource.Managed, *output, bool) error,
) error {
	if len(databaseResources) == 0 {
		out.printEmptyMessage(databaseKind, client.Project)
		return nil
	}

	if cmd.Name != "" && cmd.PrintConnectionString {
		return connectionStringHandler(ctx, client, databaseResources[0])
	}

	if cmd.Name != "" && cmd.PrintDatabaseUser {
		return cmd.printSecret(out.writer, ctx, client, databaseResources[0], func(db, _ string) string { return db })
	}

	if cmd.Name != "" && cmd.PrintPassword {
		return cmd.printSecret(out.writer, ctx, client, databaseResources[0], func(_, pw string) string { return pw })
	}

	switch out.Format {
	case full:
		return databasesHandler(databaseResources, out, true)
	case noHeader:
		return databasesHandler(databaseResources, out, false)
	case yamlOut:
		return format.PrettyPrintObjects(databaseResources, format.PrintOpts{})
	case jsonOut:
		return format.PrettyPrintObjects(
			databaseResources,
			format.PrintOpts{
				Format: format.OutputFormatTypeJSON,
				JSONOpts: format.JSONOutputOptions{
					PrintSingleItem: cmd.Name != "",
				},
			})
	}

	return nil
}

func printDatabases(out *output, databases []Database, header bool) error {
	if header {
		out.writeHeader("NAME", "FQDN", "LOCATION", "SIZE", "CONNECTIONS")
	}

	for _, db := range databases {
		out.writeTabRow(
			db.Namespace,
			db.Name,
			db.FQDN,
			db.Location,
			db.Size,
			db.Connections,
		)
	}

	return out.tabWriter.Flush()
}
