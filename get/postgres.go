package get

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type postgresCmd struct {
	resourceCmd
	PrintPassword         bool `help:"Print the password of the PostgreSQL User. Requires name to be set." xor:"print"`
	PrintUser             bool `help:"Print the name of the PostgreSQL User. Requires name to be set." xor:"print"`
	PrintConnectionString bool `help:"Print the connection string of the PostgreSQL instance. Requires name to be set." xor:"print"`

	out io.Writer
}

func (cmd *postgresCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	cmd.out = defaultOut(cmd.out)

	if cmd.Name != "" && cmd.PrintUser {
		fmt.Fprintln(cmd.out, storage.PostgresUser)
		return nil
	}

	postgresList := &storage.PostgresList{}
	if err := get.list(ctx, client, postgresList, api.MatchName(cmd.Name)); err != nil {
		return err
	}
	if len(postgresList.Items) == 0 {
		get.printEmptyMessage(cmd.out, storage.PostgresKind, client.Project)
		return nil
	}

	if cmd.Name != "" && cmd.PrintConnectionString {
		return cmd.printConnectionString(ctx, client, &postgresList.Items[0])
	}

	if cmd.Name != "" && cmd.PrintPassword {
		return cmd.printPassword(ctx, client, &postgresList.Items[0])
	}

	switch get.Output {
	case full:
		return cmd.printPostgresInstances(postgresList.Items, get, true)
	case noHeader:
		return cmd.printPostgresInstances(postgresList.Items, get, false)
	case yamlOut:
		return format.PrettyPrintObjects(postgresList.GetItems(), format.PrintOpts{})
	case jsonOut:
		return format.PrettyPrintObjects(
			postgresList.GetItems(),
			format.PrintOpts{
				Format: format.OutputFormatTypeJSON,
				JSONOpts: format.JSONOutputOptions{
					PrintSingleItem: cmd.Name != "",
				},
			})
	}

	return nil
}

func (cmd *postgresCmd) printPostgresInstances(list []storage.Postgres, get *Cmd, header bool) error {
	w := tabwriter.NewWriter(cmd.out, 0, 0, 4, ' ', 0)

	if header {
		get.writeHeader(w, "NAME", "FQDN", "LOCATION", "MACHINE TYPE")
	}

	for _, postgres := range list {
		get.writeTabRow(w, postgres.Namespace, postgres.Name, postgres.Status.AtProvider.FQDN, string(postgres.Spec.ForProvider.Location), postgres.Spec.ForProvider.MachineType.String())
	}

	return w.Flush()
}

func (cmd *postgresCmd) printPassword(ctx context.Context, client *api.Client, postgres *storage.Postgres) error {
	pw, err := getConnectionSecret(ctx, client, storage.PostgresUser, postgres)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.out, pw)
	return nil
}

// printConnectionString according to the PostgreSQL documentation:
// https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING
func (cmd *postgresCmd) printConnectionString(ctx context.Context, client *api.Client, pg *storage.Postgres) error {
	pw, err := getConnectionSecret(ctx, client, storage.PostgresUser, pg)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.out, "postgres://%s:%s@%s",
		storage.PostgresUser,
		pw,
		pg.Status.AtProvider.FQDN,
	)

	return nil
}
