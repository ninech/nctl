package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type postgresCmd struct {
	ResourceCmd
	MachineType          *string          `help:"Defines the sizing for a particular PostgreSQL instance." completion-predictor:"apifield:postgres_machine_type"`
	AllowedCidrs         *[]meta.IPv4CIDR `placeholder:"203.0.113.1/32" help:"Specifies the IP addresses allowed to connect to the instance."`
	DatabaseSSHKeysFlags `set:"ssh_keys_purpose=allowed to connect to the database server in order to up-/download and directly restore database backups"`
	KeepDailyBackups     *int `help:"Number of daily database backups to keep. Note that setting this to 0, backup will be disabled and existing dumps deleted immediately." completion-predictor:"apifield:postgres_keep_daily_backups"`
}

func (cmd *postgresCmd) Run(ctx context.Context, client *api.Client) error {
	postgres := &storage.Postgres{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}

	upd := cmd.newUpdater(client, postgres, storage.PostgresKind, func(current resource.Managed) error {
		postgres, ok := current.(*storage.Postgres)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, storage.Postgres{})
		}

		return cmd.applyUpdates(postgres)
	})

	return upd.Update(ctx)
}

func (cmd *postgresCmd) applyUpdates(postgres *storage.Postgres) error {
	if cmd.MachineType != nil {
		postgres.Spec.ForProvider.MachineType = infra.NewMachineType(*cmd.MachineType)
	}
	if cmd.AllowedCidrs != nil {
		postgres.Spec.ForProvider.AllowedCIDRs = *cmd.AllowedCidrs
	}
	if cmd.SSHKeysSet() {
		sshKeys, err := cmd.StorageKeys(&cmd.Writer)
		if err != nil {
			return err
		}
		postgres.Spec.ForProvider.SSHKeys = sshKeys
	}
	if cmd.KeepDailyBackups != nil {
		postgres.Spec.ForProvider.KeepDailyBackups = cmd.KeepDailyBackups
	}

	return nil
}
