package create

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"

	"github.com/ninech/nctl/api"
)

type postgresCmd struct {
	ResourceCmd
	Location             meta.LocationName `help:"Where the PostgreSQL instance is created." completion-predictor:"apifield:postgres_location"`
	MachineType          string            `help:"Defines the sizing for a particular PostgreSQL instance." completion-predictor:"apifield:postgres_machine_type"`
	AllowedCidrs         []meta.IPv4CIDR   `placeholder:"203.0.113.1/32" help:"IP addresses allowed to connect to the instance."`
	DatabaseSSHKeysFlags `set:"ssh_keys_purpose=allowed to connect to the database server in order to up-/download and directly restore database backups"`
	PostgresVersion      storage.PostgresVersion `help:"Release version with which the PostgreSQL instance is created." completion-predictor:"apifield:postgres_version"`
	KeepDailyBackups     *int                    `help:"Number of daily database backups to keep. Note that setting this to 0, backup will be disabled and existing dumps deleted immediately." completion-predictor:"apifield:postgres_keep_daily_backups"`
}

func (cmd *postgresCmd) Run(ctx context.Context, client *api.Client) error {
	postgres, err := cmd.newPostgres(client.Project)
	if err != nil {
		return err
	}

	c := cmd.newCreator(client, postgres, storage.PostgresKind)
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResourceInLocation(ctx, cmd.Location, func(location meta.LocationName) {
		postgres.Spec.ForProvider.Location = location
	}); err != nil {
		return err
	}

	if !cmd.Wait {
		return nil
	}

	return c.wait(ctx, waitStage{
		Writer:     cmd.Writer,
		objectList: &storage.PostgresList{},
		onResult: func(event watch.Event) (bool, error) {
			if c, ok := event.Object.(*storage.Postgres); ok {
				return isAvailable(c), nil
			}
			return false, nil
		},
	})
}

func (cmd *postgresCmd) newPostgres(namespace string) (*storage.Postgres, error) {
	name := getName(cmd.Name)

	sshKeys, err := cmd.StorageKeys(&cmd.Writer)
	if err != nil {
		return nil, err
	}

	postgres := &storage.Postgres{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: storage.PostgresSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      "postgres-" + name,
					Namespace: namespace,
				},
			},
			ForProvider: storage.PostgresParameters{
				Location:         cmd.Location,
				MachineType:      infra.NewMachineType(cmd.MachineType),
				AllowedCIDRs:     []meta.IPv4CIDR{}, // avoid missing parameter error
				SSHKeys:          sshKeys,
				Version:          cmd.PostgresVersion,
				KeepDailyBackups: cmd.KeepDailyBackups,
			},
		},
	}

	if cmd.AllowedCidrs != nil {
		postgres.Spec.ForProvider.AllowedCIDRs = cmd.AllowedCidrs
	}

	return postgres, nil
}
