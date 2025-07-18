package get

import (
	"bytes"
	"context"
	"strings"
	"testing"

	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestPostgres(t *testing.T) {
	ctx := context.Background()

	type postgresInstance struct {
		name        string
		project     string
		machineType infra.MachineType
	}

	tests := []struct {
		name      string
		instances []postgresInstance
		get       postgresCmd
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
			wantContain: []string{"no Postgres found"},
			wantLines:   1,
		},
		{
			name: "single instance in project",
			instances: []postgresInstance{
				{
					name:        "test",
					project:     test.DefaultProject,
					machineType: machineType("nine-db-prod-s"),
				},
			},
			wantContain: []string{"nine-db-prod-s"},
			wantLines:   2, // header + result
		},
		{
			name: "multiple instances in one project",
			instances: []postgresInstance{
				{
					name:        "test1",
					project:     test.DefaultProject,
					machineType: machineType("nine-db-prod-s"),
				},
				{
					name:        "test2",
					project:     test.DefaultProject,
					machineType: machineType("nine-db-prod-m"),
				},
				{
					name:        "test3",
					project:     test.DefaultProject,
					machineType: machineType("nine-db-prod-l"),
				},
			},
			wantContain: []string{"nine-db-prod-s", "nine-db-prod-m", "test3"},
			wantLines:   4, // header + result
		},
		{
			name: "multiple instances in multiple projects",
			instances: []postgresInstance{
				{
					name:        "test1",
					project:     test.DefaultProject,
					machineType: machineType("nine-db-prod-s"),
				},
				{
					name:        "test2",
					project:     "dev",
					machineType: machineType("nine-db-prod-m"),
				},
				{
					name:        "test3",
					project:     "testing",
					machineType: machineType("nine-db-prod-l"),
				},
			},
			wantContain:   []string{"test1", "test2", "test3"},
			inAllProjects: true,
			wantLines:     4, // header + result
		},
		{
			name: "get-by-name",
			instances: []postgresInstance{
				{
					name:        "test1",
					project:     test.DefaultProject,
					machineType: machineType("nine-db-prod-s"),
				},
				{
					name:        "test2",
					project:     test.DefaultProject,
					machineType: machineType("nine-db-prod-m"),
				},
			},
			get:         postgresCmd{resourceCmd: resourceCmd{Name: "test1"}},
			wantContain: []string{"test1", "nine-db-prod-s"},
			wantLines:   2,
		},
		{
			name: "show-password",
			instances: []postgresInstance{
				{
					name:        "test1",
					project:     test.DefaultProject,
					machineType: machineType("nine-db-prod-s"),
				},
				{
					name:        "test2",
					project:     test.DefaultProject,
					machineType: machineType("nine-db-prod-m"),
				},
			},
			get:         postgresCmd{resourceCmd: resourceCmd{Name: "test2"}, PrintPassword: true},
			wantContain: []string{"test2-topsecret"},
			wantLines:   1, // no header in this case
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := []client.Object{}
			for _, instance := range tt.instances {
				created := test.Postgres(instance.name, instance.project, "nine-es34")
				created.Spec.ForProvider.MachineType = instance.machineType
				objects = append(objects, created, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      created.GetWriteConnectionSecretToReference().Name,
						Namespace: created.GetWriteConnectionSecretToReference().Namespace,
					},
					Data: map[string][]byte{storage.PostgresUser: []byte(created.GetWriteConnectionSecretToReference().Name + "-topsecret")},
				})
			}
			apiClient, err := test.SetupClient(
				test.WithProjectsFromResources(objects...),
				test.WithObjects(objects...),
				test.WithNameIndexFor(&storage.Postgres{}),
				test.WithKubeconfig(t),
			)
			require.NoError(t, err)

			if tt.out == "" {
				tt.out = full
			}
			buf := &bytes.Buffer{}
			if err := tt.get.Run(ctx, apiClient, &Cmd{output: output{Format: tt.out, AllProjects: tt.inAllProjects, writer: buf}}); (err != nil) != tt.wantErr {
				t.Errorf("postgresCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			for _, substr := range tt.wantContain {
				if !strings.Contains(buf.String(), substr) {
					t.Errorf("postgresCmd.Run() did not contain %q, out = %q", tt.wantContain, buf.String())
				}
			}
			if test.CountLines(buf.String()) != tt.wantLines {
				t.Errorf("expected the output to have %d lines, but found %d", tt.wantLines, test.CountLines(buf.String()))
				t.Log(buf.String())
			}
		})
	}
}
