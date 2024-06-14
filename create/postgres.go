package create

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/alecthomas/kong"
	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/file"
)

type postgresCmd struct {
	resourceCmd
	Location         string                  `placeholder:"${postgres_location_default}" help:"Location where the PostgreSQL instance is created. Available locations are: ${postgres_location_options}"`
	MachineType      infra.MachineType       `placeholder:"${postgres_machine_default}" help:"Defines the sizing for a particular PostgreSQL instance. Available types: ${postgres_machine_types}"`
	AllowedCidrs     []meta.IPv4CIDR         `placeholder:"0.0.0.0/0" help:"Specifies the IP addresses allowed to connect to the instance." `
	SSHKeys          []storage.SSHKey        `help:"Contains a list of SSH public keys, allowed to connect to the db server, in order to up-/download and directly restore database backups."`
	SSHKeysFile      string                  `help:"Path to a file containing a list of SSH public keys (see above), separated by newlines."`
	PostgresVersion  storage.PostgresVersion `placeholder:"${postgres_version_default}" help:"Release version with which the PostgreSQL instance is created"`
	KeepDailyBackups *int                    `placeholder:"${postgres_backup_retention_days}" help:"Number of daily database backups to keep. Note that setting this to 0, backup will be disabled and existing dumps deleted immediately."`
}

func (cmd *postgresCmd) Run(ctx context.Context, client *api.Client) error {
	sshkeys, err := file.ReadSSHKeys(cmd.SSHKeysFile)
	if err != nil {
		return fmt.Errorf("error when reading SSH keys file: %w", err)
	}
	cmd.SSHKeys = append(cmd.SSHKeys, sshkeys...)

	fmt.Printf("Creating new postgres. This might take some time (waiting up to %s).\n", cmd.WaitTimeout)
	postgres := cmd.newPostgres(client.Project)

	c := newCreator(client, postgres, "postgres")
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !cmd.Wait {
		return nil
	}

	return c.wait(ctx, waitStage{
		objectList: &storage.PostgresList{},
		onResult: func(event watch.Event) (bool, error) {
			if c, ok := event.Object.(*storage.Postgres); ok {
				return isAvailable(c), nil
			}
			return false, nil
		},
	},
	)
}

func (cmd *postgresCmd) newPostgres(namespace string) *storage.Postgres {
	name := getName(cmd.Name)

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
				Location:         meta.LocationName(cmd.Location),
				MachineType:      cmd.MachineType,
				AllowedCIDRs:     []meta.IPv4CIDR{},  // avoid missing parameter error
				SSHKeys:          []storage.SSHKey{}, // avoid missing parameter error
				Version:          cmd.PostgresVersion,
				KeepDailyBackups: cmd.KeepDailyBackups,
			},
		},
	}

	if cmd.AllowedCidrs != nil {
		postgres.Spec.ForProvider.AllowedCIDRs = cmd.AllowedCidrs
	}
	if cmd.SSHKeys != nil {
		postgres.Spec.ForProvider.SSHKeys = cmd.SSHKeys
	}

	return postgres
}

// ApplicationKongVars returns all variables which are used in the application
// create command
func PostgresKongVars() kong.Vars {
	vmTypes := make([]string, len(infra.MachineTypes))
	for i, machineType := range infra.MachineTypes {
		vmTypes[i] = string(machineType)
	}

	result := make(kong.Vars)
	result["postgres_machine_types"] = strings.Join(vmTypes, ", ")
	result["postgres_machine_default"] = string(infra.MachineTypes[0])
	result["postgres_location_options"] = strings.Join(storage.PostgresLocationOptions, ", ")
	result["postgres_location_default"] = string(storage.PostgresLocationDefault)
	result["postgres_version_default"] = string(storage.PostgresVersionDefault)
	result["postgres_user"] = storage.PostgresUser
	result["postgres_backup_retention_days"] = fmt.Sprintf("%d", storage.PostgresBackupRetentionDaysDefault)

	return result
}
