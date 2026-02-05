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

// TestDatabase tests shared functionality between different database types, with a postgresdatabase
func TestDatabase(t *testing.T) {
	ctx := context.Background()

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
			name:        "simple",
			wantContain: []string{"no PostgresDatabases found"},
			wantLines:   1,
		},
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
			name: "multiple databases in one project",
			databases: []postgresDatabase{
				{
					name:     "test1",
					project:  test.DefaultProject,
					location: meta.LocationNineCZ41,
				},
				{
					name:     "test2",
					project:  test.DefaultProject,
					location: meta.LocationNineCZ42,
				},
				{
					name:     "test3",
					project:  test.DefaultProject,
					location: meta.LocationNineES34,
				},
			},
			wantContain: []string{"nine-cz41", "nine-cz42", "nine-es34"},
			wantLines:   4, // header + result
		},
		{
			name: "multiple instances in multiple projects",
			databases: []postgresDatabase{
				{
					name:     "test1",
					project:  test.DefaultProject,
					location: meta.LocationNineCZ41,
				},
				{
					name:     "test2",
					project:  "dev",
					location: meta.LocationNineCZ41,
				},
				{
					name:     "test3",
					project:  "testing",
					location: meta.LocationNineCZ41,
				},
			},
			wantContain:   []string{"test1", "test2", "test3"},
			inAllProjects: true,
			wantLines:     4, // header + result
		},
		{
			name: "get-by-name",
			databases: []postgresDatabase{
				{
					name:     "test1",
					project:  test.DefaultProject,
					location: meta.LocationNineCZ41,
				},
				{
					name:     "test2",
					project:  test.DefaultProject,
					location: meta.LocationNineCZ42,
				},
			},
			get:         postgresDatabaseCmd{databaseCmd: databaseCmd{resourceCmd: resourceCmd{Name: "test1"}}},
			wantContain: []string{"test1", "nine-cz41"},
			wantLines:   2,
		},
		{
			name: "show-password",
			databases: []postgresDatabase{
				{
					name:     "test1",
					project:  test.DefaultProject,
					location: meta.LocationNineCZ41,
				},
				{
					name:     "test2",
					project:  test.DefaultProject,
					location: meta.LocationNineCZ41,
				},
			},
			get:         postgresDatabaseCmd{databaseCmd: databaseCmd{resourceCmd: resourceCmd{Name: "test2"}, PrintPassword: true}},
			wantContain: []string{"topsecret"},
			wantLines:   1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
			require.NoError(t, err)

			if tt.out == "" {
				tt.out = full
			}
			buf := &bytes.Buffer{}
			cmd := NewTestCmd(buf, tt.out)
			cmd.AllProjects = tt.inAllProjects
			if err := tt.get.Run(ctx, apiClient, cmd); (err != nil) != tt.wantErr {
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
