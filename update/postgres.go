package update

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/file"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type postgresCmd struct {
	Name             string             `arg:"" default:"" help:"Name of the PostgreSQL instance to update."`
	MachineType      *infra.MachineType `placeholder:"${postgres_machine_default}" help:"Defines the sizing for a particular PostgreSQL instance. Available types: ${postgres_machine_types}"`
	AllowedCidrs     *[]meta.IPv4CIDR   `placeholder:"0.0.0.0/0" help:"Specifies the IP addresses allowed to connect to the instance." `
	SSHKeys          []storage.SSHKey   `help:"Contains a list of SSH public keys, allowed to connect to the db server, in order to up-/download and directly restore database backups."`
	SSHKeysFile      string             `help:"Path to a file containing a list of SSH public keys (see above), separated by newlines."`
	KeepDailyBackups *int               `placeholder:"${postgres_backup_retention_days}" help:"Number of daily database backups to keep. Note that setting this to 0, backup will be disabled and existing dumps deleted immediately."`
}

func (cmd *postgresCmd) Run(ctx context.Context, client *api.Client) error {
	postgres := &storage.Postgres{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmd.Name,
			Namespace: client.Project,
		},
	}

	upd := newUpdater(client, postgres, storage.PostgresKind, func(current resource.Managed) error {
		postgres, ok := current.(*storage.Postgres)
		if !ok {
			return fmt.Errorf("resource is of type %T, expected %T", current, storage.Postgres{})
		}

		sshkeys, err := file.ReadSSHKeys(cmd.SSHKeysFile)
		if err != nil {
			return fmt.Errorf("error when reading SSH keys file: %w", err)
		}
		cmd.SSHKeys = append(cmd.SSHKeys, sshkeys...)

		cmd.applyUpdates(postgres)
		return nil
	})

	return upd.Update(ctx)
}

func (cmd *postgresCmd) applyUpdates(postgres *storage.Postgres) {
	if cmd.MachineType != nil {
		postgres.Spec.ForProvider.MachineType = *cmd.MachineType
	}
	if cmd.AllowedCidrs != nil {
		postgres.Spec.ForProvider.AllowedCIDRs = *cmd.AllowedCidrs
	}
	if cmd.SSHKeys != nil {
		postgres.Spec.ForProvider.SSHKeys = cmd.SSHKeys
	}
	if cmd.KeepDailyBackups != nil {
		postgres.Spec.ForProvider.KeepDailyBackups = cmd.KeepDailyBackups
	}
}
