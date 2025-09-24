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

type mysqlDatabaseCmd struct {
	resourceCmd
	Location             meta.LocationName    `placeholder:"${mysqldatabase_location_default}" help:"Where the MySQL database is created. Available locations are: ${mysqldatabase_location_options}"`
	MysqlDatabaseVersion storage.MySQLVersion `placeholder:"${mysqldatabase_version_default}" help:"Version of the MySQL database. Available versions: ${mysqldatabase_versions}"`
	CharacterSet         string               `placeholder:"${mysqldatabase_characterset_default}" help:"Character set for the MySQL database. Available character sets: ${mysqldatabase_characterset_options}"`
}

func (cmd *mysqlDatabaseCmd) Run(ctx context.Context, client *api.Client) error {
	fmt.Printf("Creating new MySQLDatabase. (waiting up to %s).\n", cmd.WaitTimeout)
	mysqlDatabase := cmd.newMySQLDatabase(client.Project)

	c := newCreator(client, mysqlDatabase, "mysqldatabase")
	ctx, cancel := context.WithTimeout(ctx, cmd.WaitTimeout)
	defer cancel()

	if err := c.createResource(ctx); err != nil {
		return err
	}

	if !cmd.Wait {
		return nil
	}

	if err := c.wait(ctx, waitStage{
		objectList: &storage.MySQLDatabaseList{},
		onResult: func(event watch.Event) (bool, error) {
			if mdb, ok := event.Object.(*storage.MySQLDatabase); ok {
				return isAvailable(mdb), nil
			}
			return false, nil
		},
	}); err != nil {
		return err
	}

	fmt.Printf("\n Your MySQLDatabase %s is now available. You can retrieve the database, username and password with:\n\n nctl get mysqldatabase %s --print-connection-string\n\n", mysqlDatabase.Name, mysqlDatabase.Name)

	return nil
}

func (cmd *mysqlDatabaseCmd) newMySQLDatabase(namespace string) *storage.MySQLDatabase {
	name := getName(cmd.Name)

	mysqlDatabase := &storage.MySQLDatabase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: storage.MySQLDatabaseSpec{
			ResourceSpec: runtimev1.ResourceSpec{
				WriteConnectionSecretToReference: &runtimev1.SecretReference{
					Name:      "mysqldatabase-" + name,
					Namespace: namespace,
				},
			},
			ForProvider: storage.MySQLDatabaseParameters{
				Location: cmd.Location,
				Version:  cmd.MysqlDatabaseVersion,
				CharacterSet: storage.MySQLCharacterSet{
					Name: cmd.CharacterSet,
				},
			},
		},
	}

	return mysqlDatabase
}

// MySQLDatabaseKongVars returns all variables which are used in the MySQLDatabase
// create command
func MySQLDatabaseKongVars() kong.Vars {
	result := make(kong.Vars)
	result["mysqldatabase_location_default"] = string(storage.MySQLDatabaseLocationDefault)
	result["mysqldatabase_location_options"] = strings.Join(stringSlice(storage.MySQLDatabaseLocationOptions), ", ")
	result["mysqldatabase_version_default"] = string(storage.MySQLDatabaseVersionDefault)
	result["mysqldatabase_versions"] = strings.Join(stringSlice(storage.MySQLDatabaseVersions), ", ")
	result["mysqldatabase_characterset_default"] = "utf8mb4"
	result["mysqldatabase_characterset_options"] = strings.Join([]string{"utf8mb4"}, ", ")

	return result
}
