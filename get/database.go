package get

import (
	"context"
	"io"
	"text/tabwriter"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Database struct {
	Namespace, Name, FQDN, Location, Size, Connections string
}

type databaseCmd struct {
	resourceCmd
	PrintPassword         bool `help:"Print the password of the database. Requires name to be set." xor:"print"`
	PrintDatabaseUser     bool `help:"Print the database name and user of the database. Requires name to be set." xor:"print"`
	PrintConnectionString bool `help:"Print the connection string of the database. Requires name to be set." xor:"print"`

	out io.Writer
}

func (cmd *databaseCmd) runDatabaseCmd(ctx context.Context, client *api.Client, get *Cmd,
	databaseList runtimeclient.ObjectList, databaseResources []resource.Managed, databaseKind string,
	connectionStringHandler func(context.Context, *api.Client, resource.Managed) error,
	databasesHandler func([]resource.Managed, *Cmd, bool) error,
) error {
	cmd.out = defaultOut(cmd.out)

	if err := get.list(ctx, client, databaseList, api.MatchName(cmd.Name)); err != nil {
		return err
	}

	if len(databaseResources) == 0 {
		get.printEmptyMessage(cmd.out, databaseKind, client.Project)
		return nil
	}

	if cmd.Name != "" && cmd.PrintConnectionString {
		return connectionStringHandler(ctx, client, databaseResources[0])
	}

	if cmd.Name != "" && cmd.PrintDatabaseUser {
		return cmd.printSecret(cmd.out, ctx, client, databaseResources[0], func(db, _ string) string { return db })
	}

	if cmd.Name != "" && cmd.PrintPassword {
		return cmd.printSecret(cmd.out, ctx, client, databaseResources[0], func(_, pw string) string { return pw })
	}

	switch get.Output {
	case full:
		return databasesHandler(databaseResources, get, true)
	case noHeader:
		return databasesHandler(databaseResources, get, false)
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

func printDatabases(out io.Writer, get *Cmd, databases []Database, header bool) error {
	w := tabwriter.NewWriter(out, 0, 0, 5, ' ', 0)

	if header {
		get.writeHeader(w, "NAME", "FQDN", "LOCATION", "SIZE", "CONNECTIONS")
	}

	for _, db := range databases {
		get.writeTabRow(w,
			db.Namespace,
			db.Name,
			db.FQDN,
			db.Location,
			db.Size,
			db.Connections,
		)
	}

	return w.Flush()
}
