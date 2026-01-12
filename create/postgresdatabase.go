package create

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/alecthomas/kong"
	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"

	"github.com/ninech/nctl/api"
)

type postgresDatabaseCmd struct {
	resourceCmd
	Location                meta.LocationName       `default:"${postgresdatabase_location_default}" help:"Where the PostgreSQL database is created. Available locations are: ${postgresdatabase_location_options}"`
	PostgresDatabaseVersion storage.PostgresVersion `default:"${postgresdatabase_version_default}" help:"Release version with which the PostgreSQL database is created. Available versions: ${postgresdatabase_versions}"`
}

func (cmd *postgresDatabaseCmd) Run(ctx context.Context, client *api.Client) error {
	fmt.Printf("Creating new PostgresDatabase. (waiting up to %s).\n", cmd.WaitTimeout)
	postgresDatabase := cmd.newPostgresDatabase(client.Project)

	c := newCreator(client, postgresDatabase, "postgresdatabase")
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !cmd.Wait {
		return nil
	}

	if err := c.wait(ctx, waitStage{
		objectList: &storage.PostgresDatabaseList{},
		onResult: func(event watch.Event) (bool, error) {
			if pdb, ok := event.Object.(*storage.PostgresDatabase); ok {
				return isAvailable(pdb), nil
			}
			return false, nil
		},
	}); err != nil {
		return err
	}

	fmt.Printf("\n Your PostgresDatabase %s is now available. You can retrieve the database, username and password with:\n\n nctl get postgresdatabase %s --print-connection-string\n\n", postgresDatabase.Name, postgresDatabase.Name)

	return nil
}

func (cmd *postgresDatabaseCmd) newPostgresDatabase(namespace string) *storage.PostgresDatabase {
	name := getName(cmd.Name)

	postgresDatabase := &storage.PostgresDatabase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: storage.PostgresDatabaseSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      "postgresdatabase-" + name,
					Namespace: namespace,
				},
			},
			ForProvider: storage.PostgresDatabaseParameters{
				Location: cmd.Location,
				Version:  cmd.PostgresDatabaseVersion,
			},
		},
	}

	return postgresDatabase
}

// PostgresDatabaseKongVars returns all variables which are used in the PostgresDatabase
// create command
func PostgresDatabaseKongVars() kong.Vars {
	result := make(kong.Vars)
	result["postgresdatabase_location_default"] = string(storage.PostgresDatabaseLocationDefault)
	result["postgresdatabase_location_options"] = strings.Join(stringSlice(storage.PostgresDatabaseLocationOptions), ", ")
	result["postgresdatabase_version_default"] = string(storage.PostgresDatabaseVersionDefault)
	result["postgresdatabase_versions"] = strings.Join(stringSlice(storage.PostgresDatabaseVersions), ", ")

	return result
}
