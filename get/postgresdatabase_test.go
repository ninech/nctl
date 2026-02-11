package get

import (
	"bytes"
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

func TestPostgresDatabase(t *testing.T) {
	t.Parallel()

	type postgresDatabase struct {
		name     string
		project  string
		location meta.LocationName
	}

	tests := []struct {
		name      string
		databases []postgresDatabase
		get       postgresDatabaseCmd
		// out defines the output format and will bet set to "full" if
		// not given
		out           outputFormat
		wantContain   []string
		wantLines     int
		inAllProjects bool
		wantErr       bool
	}{
		{
			name: "single database in project",
			databases: []postgresDatabase{
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
			name: "show-connection-string",
			databases: []postgresDatabase{
				{
					name:     "test1",
					project:  test.DefaultProject,
					location: meta.LocationNineCZ41,
				},
			},
			get:         postgresDatabaseCmd{databaseCmd: databaseCmd{resourceCmd: resourceCmd{Name: "test1"}, PrintConnectionString: true}},
			wantContain: []string{"postgres://", "foo_bar", "topsecret"},
			wantLines:   1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			objects := []client.Object{}
			for _, database := range tt.databases {
				created := test.PostgresDatabase(database.name, database.project, "nine-es34")
				created.Spec.ForProvider.Location = database.location
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
				test.WithNameIndexFor(&storage.PostgresDatabase{}),
				test.WithKubeconfig(t),
			)
			is := require.New(t)
			is.NoError(err)

			if tt.out == "" {
				tt.out = full
			}
			buf := &bytes.Buffer{}
			cmd := NewTestCmd(buf, tt.out)
			cmd.AllProjects = tt.inAllProjects
			if err := tt.get.Run(t.Context(), apiClient, cmd); (err != nil) != tt.wantErr {
				t.Errorf("postgresDatabaseCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			for _, substr := range tt.wantContain {
				if !strings.Contains(buf.String(), substr) {
					t.Errorf("postgresDatabaseCmd.Run() did not contain %q, out = %q", tt.wantContain, buf.String())
				}
			}
			if test.CountLines(buf.String()) != tt.wantLines {
				t.Errorf("expected the output to have %d lines, but found %d", tt.wantLines, test.CountLines(buf.String()))
				t.Log(buf.String())
			}
		})
	}
}
