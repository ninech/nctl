package get

import (
	"bytes"
	"strings"
	"testing"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/internal/testutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TestDatabase tests shared functionality between different database types, with a postgresdatabase
func TestDatabase(t *testing.T) {
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
		// out defines the output format and will bet set to "full" if not given
		out           outputFormat
		wantContain   []string
		wantLines     int
		inAllProjects bool
		wantErr       bool
	}{
		{
			name:        "simple",
			wantErr:     true,
			wantContain: []string{`no "PostgresDatabases" found`},
		},
		{
			name: "single database in project",
			databases: []postgresDatabase{
				{
					name:     "test",
					project:  testutil.DefaultProject,
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
					project:  testutil.DefaultProject,
					location: meta.LocationNineCZ41,
				},
				{
					name:     "test2",
					project:  testutil.DefaultProject,
					location: meta.LocationNineCZ42,
				},
				{
					name:     "test3",
					project:  testutil.DefaultProject,
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
					project:  testutil.DefaultProject,
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
					project:  testutil.DefaultProject,
					location: meta.LocationNineCZ41,
				},
				{
					name:     "test2",
					project:  testutil.DefaultProject,
					location: meta.LocationNineCZ42,
				},
			},
			get:         postgresDatabaseCmd{ServiceCmd: ServiceCmd{ResourceCmd: ResourceCmd{Name: "test1"}}},
			wantContain: []string{"test1", "nine-cz41"},
			wantLines:   2,
		},
		{
			name: "show-password",
			databases: []postgresDatabase{
				{
					name:     "test1",
					project:  testutil.DefaultProject,
					location: meta.LocationNineCZ41,
				},
				{
					name:     "test2",
					project:  testutil.DefaultProject,
					location: meta.LocationNineCZ41,
				},
			},
			get:         postgresDatabaseCmd{ServiceCmd: ServiceCmd{ResourceCmd: ResourceCmd{Name: "test2"}, PrintPassword: true}},
			wantContain: []string{"topsecret"},
			wantLines:   1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			objects := []client.Object{}
			for _, database := range tt.databases {
				created := testutil.PostgresDatabase(database.name, database.project, "nine-es34")
				created.Spec.ForProvider.Location = database.location
				objects = append(objects, created, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      created.GetWriteConnectionSecretToReference().Name,
						Namespace: created.GetWriteConnectionSecretToReference().Namespace,
					},
					Data: map[string][]byte{"foo_bar": []byte("topsecret")},
				})
			}
			apiClient := testutil.SetupClient(
				t,
				testutil.WithProjectsFromResources(objects...),
				testutil.WithObjects(objects...),
				testutil.WithNameIndexFor(&storage.PostgresDatabase{}),
				testutil.WithKubeconfig(),
			)
			if tt.out == "" {
				tt.out = full
			}
			buf := &bytes.Buffer{}
			cmd := NewTestCmd(buf, tt.out)
			cmd.AllProjects = tt.inAllProjects
			err := tt.get.Run(t.Context(), apiClient, cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("postgresDatabaseCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				for _, substr := range tt.wantContain {
					if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(substr)) {
						t.Errorf("postgresDatabaseCmd.Run() error did not contain %q, err = %v", substr, err)
					}
				}
				return
			}

			for _, substr := range tt.wantContain {
				if !strings.Contains(buf.String(), substr) {
					t.Errorf("postgresDatabaseCmd.Run() did not contain %q, out = %q", tt.wantContain, buf.String())
				}
			}
			if testutil.CountLines(buf.String()) != tt.wantLines {
				t.Errorf("expected the output to have %d lines, but found %d", tt.wantLines, testutil.CountLines(buf.String()))
				t.Log(buf.String())
			}
		})
	}
}

// TestConnectionString ensures the connection strings match the DSNs injected by service connections into deplo.io applications.
func TestConnectionString(t *testing.T) {
	t.Parallel()

	withFQDN := func(fqdn string) func(resource.Managed) {
		return func(mg resource.Managed) {
			switch r := mg.(type) {
			case *storage.Postgres:
				r.Status.AtProvider.FQDN = fqdn
			case *storage.PostgresDatabase:
				r.Status.AtProvider.FQDN = fqdn
			case *storage.MySQL:
				r.Status.AtProvider.FQDN = fqdn
			case *storage.MySQLDatabase:
				r.Status.AtProvider.FQDN = fqdn
			case *storage.KeyValueStore:
				r.Status.AtProvider.FQDN = fqdn
			}
		}
	}

	tests := []struct {
		name     string
		mg       resource.Managed
		setup    func(resource.Managed)
		build    func(resource.Managed, string, string) (string, error)
		user     string
		password string
		want     string
		wantErr  bool
	}{
		{
			name:     "postgres",
			mg:       testutil.Postgres("pg", testutil.DefaultProject, "nine-es34"),
			setup:    withFQDN("pg.0000000.postgres.test.nineapis.ch"),
			build:    (&postgresCmd{}).connectionString,
			user:     storage.PostgresUser,
			password: "pgpassword",
			want:     "postgresql://dbadmin:pgpassword@pg.0000000.postgres.test.nineapis.ch:5432/postgres?sslmode=require",
		},
		{
			name:     "postgres database",
			mg:       testutil.PostgresDatabase("pgdb", testutil.DefaultProject, "nine-es34"),
			setup:    withFQDN("pgdb.0000000.postgres.test.nineapis.ch"),
			build:    (&postgresDatabaseCmd{}).connectionString,
			user:     "singledb-1234567",
			password: "pgdbpassword",
			want:     "postgresql://singledb-1234567:pgdbpassword@pgdb.0000000.postgres.test.nineapis.ch:5432/singledb-1234567?sslmode=require",
		},
		{
			name:     "mysql",
			mg:       testutil.MySQL("my", testutil.DefaultProject, "nine-es34"),
			setup:    withFQDN("testdb.0000000.mysql.test.nineapis.ch"),
			build:    (&mySQLCmd{}).connectionString,
			user:     storage.MySQLUser,
			password: "mysqlpassword",
			want:     "mysql://dbadmin:mysqlpassword@testdb.0000000.mysql.test.nineapis.ch:3306?ssl-mode=REQUIRED",
		},
		{
			name:     "mysql database",
			mg:       testutil.MySQLDatabase("mydb", testutil.DefaultProject, "nine-es34"),
			setup:    withFQDN("testdb.0000000.mysql.test.nineapis.ch"),
			build:    (&mysqlDatabaseCmd{}).connectionString,
			user:     "singledb-1234567",
			password: "mysqldbpassword",
			want:     "mysql://singledb-1234567:mysqldbpassword@testdb.0000000.mysql.test.nineapis.ch:3306/singledb-1234567?ssl-mode=REQUIRED",
		},
		{
			name:     "keyvaluestore",
			mg:       testutil.KeyValueStore("kvs", testutil.DefaultProject, meta.LocationNineES34),
			setup:    withFQDN("testdb.0000000.keyvaluestore.test.nineapis.ch"),
			build:    (&keyValueStoreCmd{}).connectionString,
			user:     storage.KeyValueStoreUser,
			password: "kvspassword",
			want:     "rediss://default:kvspassword@testdb.0000000.keyvaluestore.test.nineapis.ch:6379",
		},
		{
			name: "opensearch",
			mg:   testutil.OpenSearch("os", testutil.DefaultProject, meta.LocationNineES34),
			setup: func(mg resource.Managed) {
				mg.(*storage.OpenSearch).Status.AtProvider.URL = "https://testdb.0000000.opensearch.test.nineapis.ch:9200"
			},
			build:    (&openSearchCmd{}).connectionString,
			user:     storage.OpenSearchUser,
			password: "opensearchpassword",
			want:     "https://admin:opensearchpassword@testdb.0000000.opensearch.test.nineapis.ch:9200",
		},
		{
			name:     "password with reserved characters",
			mg:       testutil.KeyValueStore("kvs", testutil.DefaultProject, meta.LocationNineES34),
			setup:    withFQDN("kvs.example.com"),
			build:    (&keyValueStoreCmd{}).connectionString,
			user:     storage.KeyValueStoreUser,
			password: "p@ss:w/rd?#%",
			want:     "rediss://default:p%40ss%3Aw%2Frd%3F%23%25@kvs.example.com:6379",
		},
		{
			name:     "missing fqdn",
			mg:       testutil.KeyValueStore("kvs", testutil.DefaultProject, meta.LocationNineES34),
			setup:    withFQDN(""),
			build:    (&keyValueStoreCmd{}).connectionString,
			user:     storage.KeyValueStoreUser,
			password: "kvspassword",
			wantErr:  true,
		},
		{
			name:     "missing opensearch url",
			mg:       testutil.OpenSearch("os", testutil.DefaultProject, meta.LocationNineES34),
			setup:    func(resource.Managed) {},
			build:    (&openSearchCmd{}).connectionString,
			user:     storage.OpenSearchUser,
			password: "opensearchpassword",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tt.setup(tt.mg)
			got, err := tt.build(tt.mg, tt.user, tt.password)
			if (err != nil) != tt.wantErr {
				t.Fatalf("connectionString() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("connectionString() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCredentials(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		secrets      map[string][]byte
		wantUser     string
		wantPassword string
		wantErr      bool
	}{
		{
			name:         "single user",
			secrets:      map[string][]byte{"dbadmin": []byte("topsecret")},
			wantUser:     "dbadmin",
			wantPassword: "topsecret",
		},
		{
			name:    "empty",
			secrets: map[string][]byte{},
			wantErr: true,
		},
		{
			name:    "ambiguous",
			secrets: map[string][]byte{"a": []byte("1"), "b": []byte("2")},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			user, pw, err := credentials(tt.secrets)
			if (err != nil) != tt.wantErr {
				t.Fatalf("credentials() error = %v, wantErr %v", err, tt.wantErr)
			}
			if user != tt.wantUser || pw != tt.wantPassword {
				t.Errorf("credentials() = %q, %q, want %q, %q", user, pw, tt.wantUser, tt.wantPassword)
			}
		})
	}
}
