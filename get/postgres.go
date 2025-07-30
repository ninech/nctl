package get

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type postgresCmd struct{ databaseCmd }

func (cmd *postgresCmd) Run(ctx context.Context, c *api.Client, get *Cmd) error {
	return get.listPrint(ctx, c, cmd, api.MatchName(cmd.Name))
}

func (cmd *postgresCmd) list() client.ObjectList {
	return &storage.PostgresList{}
}

func (cmd *postgresCmd) print(ctx context.Context, client *api.Client, list client.ObjectList, out *output) error {
	databaseList, ok := list.(*storage.PostgresList)
	if !ok {
		return fmt.Errorf("expected %T, got %T", &storage.PostgresList{}, list)
	}

	return cmd.run(ctx, client, &Cmd{output: *out},
		databaseList, storage.PostgresKind,
		cmd.printConnectionString,
		cmd.printPostgresInstances,
		func(mg resource.Managed) (string, error) {
			db, ok := mg.(*storage.Postgres)
			if !ok {
				return "", fmt.Errorf("expected postgres, got %T", mg)
			}
			return db.Status.AtProvider.CACert, nil
		},
	)
}

func (cmd *postgresCmd) printPostgresInstances(resources []resource.Managed, get *Cmd, header bool) error {
	if header {
		get.writeHeader("NAME", "FQDN", "LOCATION", "MACHINE TYPE")
	}

	for _, mg := range resources {
		db, ok := mg.(*storage.Postgres)
		if !ok {
			return fmt.Errorf("expected postgres, got %T", mg)
		}

		get.writeTabRow(db.Namespace, db.Name, db.Status.AtProvider.FQDN, string(db.Spec.ForProvider.Location), db.Spec.ForProvider.MachineType.String())
	}

	return get.tabWriter.Flush()
}

func (cmd *postgresCmd) printConnectionString(ctx context.Context, client *api.Client, mg resource.Managed) error {
	my, ok := mg.(*storage.Postgres)
	if !ok {
		return fmt.Errorf("expected postgres, got %T", mg)
	}

	secrets, err := getConnectionSecretMap(ctx, client, my)
	if err != nil {
		return err
	}

	for db, pw := range secrets {
		fmt.Fprintln(cmd.out, postgresConnectionString(my.Status.AtProvider.FQDN, db, db, pw))
		return nil
	}

	return nil
}
