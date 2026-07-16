package copy

import (
	"context"
	"fmt"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type postgresDatabaseCmd struct {
	databaseCmd
}

func (cmd *postgresDatabaseCmd) Run(ctx context.Context, client *api.Client) error {
	source := &storage.PostgresDatabase{}
	if err := client.Get(ctx, client.Name(cmd.Name), source); err != nil {
		return fmt.Errorf("unable to get postgresdatabase %q: %w", cmd.Name, err)
	}

	newDB := cmd.newTarget(source, client.Project)
	newDB.Spec.ForProvider.CloneFrom = &meta.LocalReference{Name: source.Name}
	if cmd.TargetVersion != "" {
		newDB.Spec.ForProvider.Version = storage.PostgresVersion(cmd.TargetVersion)
	}

	return cmd.run(ctx, client, databaseCopy{
		kind:  storage.PostgresDatabaseKind,
		newDB: newDB,
		cloneFromSet: func(mg resource.Managed) bool {
			db, ok := mg.(*storage.PostgresDatabase)
			return ok && db.Spec.ForProvider.CloneFrom != nil
		},
	})
}

// newTarget returns the new database, inheriting the spec of the source.
// cloneFrom is set by Run. Any copy or instance reference inherited from the
// source spec is cleared.
func (cmd *postgresDatabaseCmd) newTarget(source *storage.PostgresDatabase, project string) *storage.PostgresDatabase {
	name := getName(cmd.TargetName)
	newDB := &storage.PostgresDatabase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		Spec: source.Spec,
	}
	// the instance reference is managed by the backend and must not be copied
	newDB.Spec.ForProvider.InstanceReference = nil
	// clear any copy reference inherited from the source (it is creation-only)
	newDB.Spec.ForProvider.RestoreFrom = nil
	newDB.Spec.ForProvider.CloneFrom = nil
	newDB.Spec.ResourceSpec = runtimev1.ResourceSpec{
		WriteConnectionSecretToReference: &runtimev1.SecretReference{
			Name:      "postgresdatabase-" + name,
			Namespace: project,
		},
	}

	return newDB
}
