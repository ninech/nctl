package get

import (
	"bytes"
	"strings"
	"testing"

	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/internal/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestMySQL(t *testing.T) {
	t.Parallel()

	type mysqlInstance struct {
		name        string
		project     string
		machineType infra.MachineType
	}

	tests := []struct {
		name      string
		instances []mysqlInstance
		get       mySQLCmd
		// out defines the output format and will bet set to "full" if
		// not given
		out           outputFormat
		inAllProjects bool
		wantContain   []string
		wantLines     int
		wantErr       bool
	}{
		{
			name:        "simple",
			wantErr:     true,
			wantContain: []string{`no "MySQLs" found`},
		},
		{
			name: "single instance in project",
			instances: []mysqlInstance{
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
			name: "multiple instances in same project",
			instances: []mysqlInstance{
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
			name: "get-by-name",
			instances: []mysqlInstance{
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
			get:         mySQLCmd{databaseCmd{resourceCmd: resourceCmd{Name: "test1"}}},
			wantContain: []string{"test1", "nine-db-prod-s"},
			wantLines:   2, // header + result
		},
		{
			name: "multiple instances in multiple projects",
			instances: []mysqlInstance{
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
			inAllProjects: true,
			wantContain:   []string{"test1", "test2", "test3"},
			wantLines:     4, // header + result
		},
		{
			name: "show-password",
			instances: []mysqlInstance{
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
			get:         mySQLCmd{databaseCmd{resourceCmd: resourceCmd{Name: "test2"}, PrintPassword: true}},
			wantContain: []string{"test2-topsecret"},
			wantLines:   1, // here no header gets printed
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			objects := []client.Object{}
			for _, instance := range tt.instances {
				created := test.MySQL(instance.name, instance.project, "nine-es34")
				created.Spec.ForProvider.MachineType = instance.machineType
				objects = append(objects, created, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      created.GetWriteConnectionSecretToReference().Name,
						Namespace: created.GetWriteConnectionSecretToReference().Namespace,
					},
					Data: map[string][]byte{storage.MySQLUser: []byte(created.GetWriteConnectionSecretToReference().Name + "-topsecret")},
				})
			}

			apiClient := test.SetupClient(t,
				test.WithProjectsFromResources(objects...),
				test.WithObjects(objects...),
				test.WithNameIndexFor(&storage.MySQL{}),
				test.WithKubeconfig(),
			)
			if tt.out == "" {
				tt.out = full
			}
			buf := &bytes.Buffer{}
			cmd := NewTestCmd(buf, tt.out)
			cmd.AllProjects = tt.inAllProjects
			err := tt.get.Run(t.Context(), apiClient, cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("mySQLCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				for _, substr := range tt.wantContain {
					if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(substr)) {
						t.Errorf("mySQLCmd.Run() error did not contain %q, err = %v", substr, err)
					}
				}
				return
			}

			for _, substr := range tt.wantContain {
				if !strings.Contains(buf.String(), substr) {
					t.Errorf("mySQLCmd.Run() did not contain %q, out = %q", tt.wantContain, buf.String())
				}
			}
			if test.CountLines(buf.String()) != tt.wantLines {
				t.Errorf("expected the output to have %d lines, but found %d", tt.wantLines, test.CountLines(buf.String()))
				t.Log(buf.String())
			}
		})
	}
}

func machineType(name string) infra.MachineType {
	return infra.NewMachineType(name)
}
