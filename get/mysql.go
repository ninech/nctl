package get

import (
	"context"
	"fmt"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type mySQLCmd struct {
	resourceCmd
	PrintPassword         bool `help:"Print the password of the MySQL User. Requires name to be set." xor:"print"`
	PrintUser             bool `help:"Print the name of the MySQL User. Requires name to be set." xor:"print"`
	PrintConnectionString bool `help:"Print the connection string of the MySQL instance. Requires name to be set." xor:"print"`
}

func (cmd *mySQLCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	if cmd.Name != "" && cmd.PrintUser {
		fmt.Fprintln(get.writer, storage.MySQLUser)
		return nil
	}

	return get.listPrint(ctx, client, cmd, api.MatchName(cmd.Name))
}

func (cmd *mySQLCmd) list() runtimeclient.ObjectList {
	return &storage.MySQLList{}
}

func (cmd *mySQLCmd) print(ctx context.Context, client *api.Client, list runtimeclient.ObjectList, out *output) error {
	mysqlList := list.(*storage.MySQLList)
	if len(mysqlList.Items) == 0 {
		out.printEmptyMessage(storage.MySQLKind, client.Project)
		return nil
	}

	if cmd.Name != "" && cmd.PrintConnectionString {
		return cmd.printConnectionString(ctx, client, &mysqlList.Items[0], out)
	}

	if cmd.Name != "" && cmd.PrintPassword {
		return cmd.printPassword(ctx, client, &mysqlList.Items[0], out)
	}

	switch out.Format {
	case full:
		return cmd.printMySQLInstances(mysqlList.Items, out, true)
	case noHeader:
		return cmd.printMySQLInstances(mysqlList.Items, out, false)
	case yamlOut:
		return format.PrettyPrintObjects(mysqlList.GetItems(), format.PrintOpts{})
	case jsonOut:
		return format.PrettyPrintObjects(
			mysqlList.GetItems(),
			format.PrintOpts{
				Format: format.OutputFormatTypeJSON,
				JSONOpts: format.JSONOutputOptions{
					PrintSingleItem: cmd.Name != "",
				},
			})
	}

	return nil
}

func (cmd *mySQLCmd) printMySQLInstances(list []storage.MySQL, out *output, header bool) error {
	if header {
		out.writeHeader("NAME", "FQDN", "LOCATION", "MACHINE TYPE")
	}

	for _, mysql := range list {
		out.writeTabRow(mysql.Namespace, mysql.Name, mysql.Status.AtProvider.FQDN, string(mysql.Spec.ForProvider.Location), mysql.Spec.ForProvider.MachineType.String())
	}

	return out.tabWriter.Flush()
}

func (cmd *mySQLCmd) printPassword(ctx context.Context, client *api.Client, mysql *storage.MySQL, out *output) error {
	pw, err := getConnectionSecret(ctx, client, storage.MySQLUser, mysql)
	if err != nil {
		return err
	}

	_, err = fmt.Fprintln(out.writer, pw)
	return err
}

// printConnectionString according to the MySQL documentation:
// https://dev.mysql.com/doc/refman/8.4/en/connecting-using-uri-or-key-value-pairs.html#connecting-using-uri
func (cmd *mySQLCmd) printConnectionString(ctx context.Context, client *api.Client, mysql *storage.MySQL, out *output) error {
	pw, err := getConnectionSecret(ctx, client, storage.MySQLUser, mysql)
	if err != nil {
		return err
	}

	fmt.Fprintf(out.writer, "mysql://%s:%s@%s",
		storage.MySQLUser,
		pw,
		mysql.Status.AtProvider.FQDN,
	)

	return nil
}
