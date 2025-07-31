package get

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type mySQLCmd struct{ databaseCmd }

func (cmd *mySQLCmd) Run(ctx context.Context, c *api.Client, get *Cmd) error {
	return get.listPrint(ctx, c, cmd, api.MatchName(cmd.Name))
}

func (cmd *mySQLCmd) list() client.ObjectList {
	return &storage.MySQLList{}
}

func (cmd *mySQLCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	databaseList, ok := list.(*storage.MySQLList)
	if !ok {
		return fmt.Errorf("expected %T, got %T", &storage.MySQLList{}, list)
	}

	return cmd.run(ctx, client, &Cmd{output: *out},
		databaseList, storage.MySQLKind,
		cmd.printConnectionString,
		cmd.printMySQLInstances,
		func(mg resource.Managed) (string, error) {
			db, ok := mg.(*storage.MySQL)
			if !ok {
				return "", fmt.Errorf("expected %T, got %T", &storage.MySQL{}, mg)
			}
			return db.Status.AtProvider.CACert, nil
		},
	)
}

func (cmd *mySQLCmd) printMySQLInstances(resources resource.ManagedList, get *Cmd, header bool) error {
	dbs, ok := resources.(*storage.MySQLList)
	if !ok {
		return fmt.Errorf("expected %T, got %T", &storage.MySQLList{}, dbs)
	}

	if header {
		get.writeHeader("NAME", "FQDN", "LOCATION", "MACHINE TYPE")
	}

	for _, db := range dbs.Items {
		get.writeTabRow(db.Namespace, db.Name, db.Status.AtProvider.FQDN, string(db.Spec.ForProvider.Location), db.Spec.ForProvider.MachineType.String())
	}

	return get.tabWriter.Flush()
}

func (cmd *mySQLCmd) printConnectionString(ctx context.Context, client *api.Client, mg resource.Managed) error {
	my, ok := mg.(*storage.MySQL)
	if !ok {
		return fmt.Errorf("expected mysql, got %T", mg)
	}

	secrets, err := getConnectionSecretMap(ctx, client, my)
	if err != nil {
		return err
	}

	for db, pw := range secrets {
		fmt.Fprintln(cmd.out, mySQLConnectionString(my.Status.AtProvider.FQDN, db, db, pw))
		return nil
	}

	return nil
}
