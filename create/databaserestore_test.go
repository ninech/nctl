package create

import (
	"testing"
	"time"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDatabaseRestore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		backup   string
		target   string
		objects  []runtimeclient.Object
		wantErr  bool
		wantKind string
	}{
		{
			name: "postgres", backup: "mybackup", target: "postgresdatabase/mydb", wantKind: storage.PostgresDatabaseKind,
			objects: []runtimeclient.Object{test.PostgresDatabase("mydb", test.DefaultProject, "nine-es34")},
		},
		{
			name: "mysql", backup: "mybackup", target: "mysqldatabase/mydb", wantKind: storage.MySQLDatabaseKind,
			objects: []runtimeclient.Object{test.MySQLDatabase("mydb", test.DefaultProject, "nine-es34")},
		},
		{name: "invalidTarget", backup: "mybackup", target: "mydb", wantErr: true},
		{name: "targetMissing", backup: "mybackup", target: "postgresdatabase/mydb", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			cmd := databaseRestoreCmd{Backup: tt.backup, Target: tt.target}
			cmd.Name = "test-" + tt.name
			cmd.Wait = false
			cmd.WaitTimeout = time.Second

			apiClient := test.SetupClient(t, test.WithObjects(tt.objects...))

			err := cmd.Run(t.Context(), apiClient)
			if tt.wantErr {
				is.Error(err)
				return
			}
			is.NoError(err)

			created := &storage.DatabaseRestore{ObjectMeta: metav1.ObjectMeta{Name: cmd.Name, Namespace: apiClient.Project}}
			is.NoError(apiClient.Get(t.Context(), api.ObjectName(created), created))
			is.Equal(tt.backup, created.Spec.ForProvider.Backup.Name)
			is.Equal("mydb", created.Spec.ForProvider.Target.Name)
			is.Equal(tt.wantKind, created.Spec.ForProvider.Target.Kind)
		})
	}
}
