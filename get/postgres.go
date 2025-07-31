package get

import (
	"context"
	"fmt"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type postgresCmd struct {
	resourceCmd
	PrintPassword         bool `help:"Print the password of the PostgreSQL User. Requires name to be set." xor:"print"`
	PrintUser             bool `help:"Print the name of the PostgreSQL User. Requires name to be set." xor:"print"`
	PrintConnectionString bool `help:"Print the connection string of the PostgreSQL instance. Requires name to be set." xor:"print"`
}

func (cmd *postgresCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	if cmd.Name != "" && cmd.PrintUser {
		fmt.Fprintln(get.writer, storage.PostgresUser)
		return nil
	}

	return get.listPrint(ctx, client, cmd, api.MatchName(cmd.Name))
}

func (cmd *postgresCmd) list() runtimeclient.ObjectList {
	return &storage.PostgresList{}
}

func (cmd *postgresCmd) print(ctx context.Context, client *api.Client, list runtimeclient.ObjectList, out *output) error {
	postgresList := list.(*storage.PostgresList)
	if len(postgresList.Items) == 0 {
		out.printEmptyMessage(storage.PostgresKind, client.Project)
		return nil
	}

	if cmd.Name != "" && cmd.PrintConnectionString {
		return cmd.printConnectionString(ctx, client, &postgresList.Items[0], out)
	}

	if cmd.Name != "" && cmd.PrintPassword {
		return cmd.printPassword(ctx, client, &postgresList.Items[0], out)
	}

	switch out.Format {
	case full:
		return cmd.printPostgresInstances(postgresList.Items, out, true)
	case noHeader:
		return cmd.printPostgresInstances(postgresList.Items, out, false)
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

func (cmd *postgresCmd) printPostgresInstances(list []storage.Postgres, out *output, header bool) error {
	if header {
		out.writeHeader("NAME", "FQDN", "LOCATION", "MACHINE TYPE")
	}

	for _, postgres := range list {
		out.writeTabRow(postgres.Namespace, postgres.Name, postgres.Status.AtProvider.FQDN, string(postgres.Spec.ForProvider.Location), postgres.Spec.ForProvider.MachineType.String())
	}

	return out.tabWriter.Flush()
}

func (cmd *postgresCmd) printPassword(ctx context.Context, client *api.Client, postgres *storage.Postgres, out *output) error {
	pw, err := getConnectionSecret(ctx, client, storage.PostgresUser, postgres)
	if err != nil {
		return err
	}

	fmt.Fprintln(out.writer, pw)
	return nil
}

// printConnectionString according to the PostgreSQL documentation:
// https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING
func (cmd *postgresCmd) printConnectionString(ctx context.Context, client *api.Client, pg *storage.Postgres, out *output) error {
	pw, err := getConnectionSecret(ctx, client, storage.PostgresUser, pg)
	if err != nil {
		return err
	}

	fmt.Fprintf(out.writer, "postgres://%s:%s@%s",
		storage.PostgresUser,
		pw,
		pg.Status.AtProvider.FQDN,
	)

	return nil
}
