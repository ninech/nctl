package get

import (
	"context"
	"fmt"
	"strconv"

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
		cmd.connectionString,
		cmd.printPostgresInstances,
		func(mg resource.Managed) (string, error) {
			db, ok := mg.(*storage.Postgres)
			if !ok {
				return "", fmt.Errorf("expected %T, got %T", &storage.Postgres{}, mg)
			}
			return db.Status.AtProvider.CACert, nil
		},
	)
}

func (cmd *postgresCmd) printPostgresInstances(resources resource.ManagedList, get *Cmd, header bool) error {
	dbs, ok := resources.(*storage.PostgresList)
	if !ok {
		return fmt.Errorf("expected %T, got %T", &storage.PostgresList{}, dbs)
	}

	if header {
		get.writeHeader("NAME", "LOCATION", "VERSION", "FQDN", "MACHINE TYPE", "DISK SIZE", "DATABASES")
	}

	for _, db := range dbs.Items {
		get.writeTabRow(db.Namespace, db.Name, string(db.Spec.ForProvider.Location), string(db.Spec.ForProvider.Version), db.Status.AtProvider.FQDN, db.Spec.ForProvider.MachineType.String(), db.Status.AtProvider.Size.String(), strconv.Itoa(len(db.Status.AtProvider.Databases)))
	}

	return get.tabWriter.Flush()
}

func (cmd *postgresCmd) connectionString(mg resource.Managed, secrets map[string][]byte) (string, error) {
	my, ok := mg.(*storage.Postgres)
	if !ok {
		return "", fmt.Errorf("expected %T, got %T", &storage.Postgres{}, mg)
	}

	for user, pw := range secrets {
		return postgresConnectionString(my.Status.AtProvider.FQDN, user, "postgres", pw), nil
	}

	return "", nil
}
