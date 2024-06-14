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

type mySQLCmd struct {
	resourceCmd
	PrintPassword bool `help:"Print the password of the MySQL User. Requires name to be set." xor:"print"`
	PrintUser     bool `help:"Print the name of the MySQL User. Requires name to be set." xor:"print"`

	out io.Writer
}

func (cmd *mySQLCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	cmd.out = defaultOut(cmd.out)

	if cmd.Name != "" && cmd.PrintUser {
		fmt.Fprintln(cmd.out, storage.MySQLUser)
		return nil
	}

	mysqlList := &storage.MySQLList{}

	if err := get.list(ctx, client, mysqlList, matchName(cmd.Name)); err != nil {
		return err
	}

	if len(mysqlList.Items) == 0 {
		printEmptyMessage(cmd.out, storage.MySQLKind, client.Project)
		return nil
	}

	if cmd.Name != "" && cmd.PrintPassword {
		return cmd.printPassword(ctx, client, &mysqlList.Items[0])
	}

	switch get.Output {
	case full:
		return cmd.printMySQLInstances(mysqlList.Items, get, true)
	case noHeader:
		return cmd.printMySQLInstances(mysqlList.Items, get, false)
	case yamlOut:
		return format.PrettyPrintObjects(mysqlList.GetItems(), format.PrintOpts{})
	}

	return nil
}

func (cmd *mySQLCmd) printMySQLInstances(list []storage.MySQL, get *Cmd, header bool) error {
	w := tabwriter.NewWriter(cmd.out, 0, 0, 4, ' ', 0)

	if header {
		get.writeHeader(w, "NAME", "FQDN", "LOCATION", "MACHINE TYPE")
	}

	for _, mysql := range list {
		get.writeTabRow(w, mysql.Namespace, mysql.Name, mysql.Status.AtProvider.FQDN, string(mysql.Spec.ForProvider.Location), string(mysql.Spec.ForProvider.MachineType))
	}

	return w.Flush()
}

func (cmd *mySQLCmd) printPassword(ctx context.Context, client *api.Client, mysql *storage.MySQL) error {
	pw, err := getConnectionSecret(ctx, client, storage.MySQLUser, mysql)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.out, pw)
	return nil
}
