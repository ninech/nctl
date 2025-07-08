package get

import (
	"bytes"
	"context"
	"strings"
	"testing"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestMySQLDatabase(t *testing.T) {
	ctx := context.Background()

	type mysqlDatabase struct {
		name         string
		project      string
		location     meta.LocationName
		characterSet storage.MySQLCharacterSet
	}

	tests := []struct {
		name      string
		databases []mysqlDatabase
		get       mysqlDatabaseCmd
		// out defines the output format and will bet set to "full" if
		// not given
		out           output
		wantContain   []string
		wantLines     int
		inAllProjects bool
		wantErr       bool
	}{
		{
			name: "single database in project",
			databases: []mysqlDatabase{
				{
					name:     "test",
					project:  test.DefaultProject,
					location: meta.LocationNineCZ41,
				},
			},
			wantContain: []string{"nine-cz41"},
			wantLines:   2, // header + result
		},
		{
			name: "show-character-set",
			databases: []mysqlDatabase{
				{
					name:         "test1",
					project:      test.DefaultProject,
					location:     meta.LocationNineCZ41,
					characterSet: storage.MySQLCharacterSet{Name: "utf8mb4"},
				},
				{
					name:         "test2",
					project:      test.DefaultProject,
					location:     meta.LocationNineCZ41,
					characterSet: storage.MySQLCharacterSet{Name: "ascii"},
				},
			},
			get:         mysqlDatabaseCmd{PrintCharacterSet: true, databaseCmd: databaseCmd{resourceCmd: resourceCmd{Name: "test2"}}},
			wantContain: []string{"ascii"},
			wantLines:   1,
		},
		{
			name: "show-connection-string",
			databases: []mysqlDatabase{
				{
					name:     "test1",
					project:  test.DefaultProject,
					location: meta.LocationNineCZ41,
				},
			},
			get:         mysqlDatabaseCmd{databaseCmd: databaseCmd{resourceCmd: resourceCmd{Name: "test1"}, PrintConnectionString: true}},
			wantContain: []string{"mysql://", "foo_bar", "topsecret"},
			wantLines:   1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			tt.get.out = buf

			objects := []client.Object{}
			for _, database := range tt.databases {
				created := test.MySQLDatabase(database.name, database.project, "nine-es34")
				created.Spec.ForProvider.Location = database.location
				created.Spec.ForProvider.CharacterSet = database.characterSet
				objects = append(objects, created, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      created.GetWriteConnectionSecretToReference().Name,
						Namespace: created.GetWriteConnectionSecretToReference().Namespace,
					},
					Data: map[string][]byte{"foo_bar": []byte("topsecret")},
				})
			}
			apiClient, err := test.SetupClient(
				test.WithProjectsFromResources(objects...),
				test.WithObjects(objects...),
				test.WithNameIndexFor(&storage.MySQLDatabase{}),
				test.WithKubeconfig(t),
			)
			require.NoError(t, err)

			if tt.out == "" {
				tt.out = full
			}
			if err := tt.get.Run(ctx, apiClient, &Cmd{Output: tt.out, AllProjects: tt.inAllProjects}); (err != nil) != tt.wantErr {
				t.Errorf("mysqlDatabaseCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			for _, substr := range tt.wantContain {
				if !strings.Contains(buf.String(), substr) {
					t.Errorf("mysqlDatabaseCmd.Run() did not contain %q, out = %q", tt.wantContain, buf.String())
				}
			}
			if test.CountLines(buf.String()) != tt.wantLines {
				t.Errorf("expected the output to have %d lines, but found %d", tt.wantLines, test.CountLines(buf.String()))
				t.Log(buf.String())
			}
		})
	}
}
