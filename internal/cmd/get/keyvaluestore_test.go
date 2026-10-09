package get

import (
	"bytes"
	"strings"
	"testing"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/internal/testutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestKeyValueStore(t *testing.T) {
	t.Parallel()

	type kvsInstance struct {
		name    string
		project string
		memSize *storage.KeyValueStoreMemorySize
	}

	tests := []struct {
		name          string
		instances     []kvsInstance
		get           keyValueStoreCmd
		out           outputFormat
		inAllProjects bool
		wantContain   []string
		wantLines     int
		wantErr       bool
	}{
		{
			name:        "simple",
			get:         keyValueStoreCmd{},
			out:         full,
			wantErr:     true,
			wantContain: []string{`no "KeyValueStores" found`},
		},
		{
			name: "single",
			instances: []kvsInstance{
				{
					name:    "test",
					project: testutil.DefaultProject,
					memSize: kvsMem("1G"),
				},
			},
			get:         keyValueStoreCmd{},
			out:         full,
			wantContain: []string{"1G"},
			wantLines:   2, // header + result
		},
		{
			name: "multiple in same project",
			instances: []kvsInstance{
				{
					name:    "test1",
					project: testutil.DefaultProject,
					memSize: kvsMem("1G"),
				},
				{
					name:    "test2",
					project: testutil.DefaultProject,
					memSize: kvsMem("2G"),
				},
				{
					name:    "test3",
					project: testutil.DefaultProject,
					memSize: kvsMem("3G"),
				},
			},
			get:         keyValueStoreCmd{},
			out:         full,
			wantContain: []string{"1G", "2G", "test3"},
			wantLines:   4, // header + result
		},
		{
			name: "get specific instance",
			instances: []kvsInstance{
				{
					name:    "test1",
					project: testutil.DefaultProject,
					memSize: kvsMem("1G"),
				},
				{
					name:    "test2",
					project: testutil.DefaultProject,
					memSize: kvsMem("2G"),
				},
			},
			get:         keyValueStoreCmd{ServiceCmd: ServiceCmd{ResourceCmd: ResourceCmd{Name: "test1"}}},
			out:         full,
			wantContain: []string{"test1", "1G"},
			wantLines:   2, // header + result
		},
		{
			name: "multiple instances in multiple projects",
			instances: []kvsInstance{
				{
					name:    "test1",
					project: testutil.DefaultProject,
					memSize: kvsMem("1G"),
				},
				{
					name:    "test2",
					project: "dev",
					memSize: kvsMem("2G"),
				},
				{
					name:    "prod1",
					project: "prod",
					memSize: kvsMem("3G"),
				},
			},
			get:           keyValueStoreCmd{},
			out:           full,
			wantContain:   []string{"test1", "test2", "prod1"},
			wantLines:     4,
			inAllProjects: true,
		},
		{
			name: "get password",
			instances: []kvsInstance{
				{
					name:    "test1",
					project: testutil.DefaultProject,
					memSize: kvsMem("1G"),
				},
				{
					name:    "test2",
					project: testutil.DefaultProject,
					memSize: kvsMem("2G"),
				},
			},
			get:         keyValueStoreCmd{ServiceCmd: ServiceCmd{ResourceCmd: ResourceCmd{Name: "test2"}, PrintPassword: true}},
			out:         full,
			wantContain: []string{"test2-topsecret"},
			wantLines:   1, // print password does not print any header line
		},
		{
			name:        "get password with deprecated token flag",
			instances:   []kvsInstance{{name: "test1", project: testutil.DefaultProject, memSize: kvsMem("1G")}},
			get:         keyValueStoreCmd{ServiceCmd: ServiceCmd{ResourceCmd: ResourceCmd{Name: "test1"}}, PrintToken: true},
			out:         full,
			wantContain: []string{"test1-topsecret"},
			wantLines:   1,
		},
		{
			name:        "get user",
			instances:   []kvsInstance{{name: "test1", project: testutil.DefaultProject, memSize: kvsMem("1G")}},
			get:         keyValueStoreCmd{ServiceCmd: ServiceCmd{ResourceCmd: ResourceCmd{Name: "test1"}, PrintUser: true}},
			out:         full,
			wantContain: []string{storage.KeyValueStoreUser},
			wantLines:   1,
		},
		{
			name:        "get connection string",
			instances:   []kvsInstance{{name: "test1", project: testutil.DefaultProject, memSize: kvsMem("1G")}},
			get:         keyValueStoreCmd{ServiceCmd: ServiceCmd{ResourceCmd: ResourceCmd{Name: "test1"}, PrintConnectionString: true}},
			out:         full,
			wantContain: []string{"rediss://default:test1-topsecret@test1.example.com:6379"},
			wantLines:   1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			objects := []client.Object{}
			for _, instance := range tt.instances {
				created := testutil.KeyValueStore(instance.name, instance.project, "nine-es34")
				created.Spec.ForProvider.MemorySize = instance.memSize
				created.Status.AtProvider.FQDN = instance.name + ".example.com"
				objects = append(objects, created)
				objects = append(objects, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      created.GetWriteConnectionSecretToReference().Name,
						Namespace: created.GetWriteConnectionSecretToReference().Namespace,
					},
					Data: map[string][]byte{storage.KeyValueStoreUser: []byte(created.GetWriteConnectionSecretToReference().Name + "-topsecret")},
				})
			}
			apiClient := testutil.SetupClient(
				t,
				testutil.WithProjectsFromResources(objects...),
				testutil.WithObjects(objects...),
				testutil.WithNameIndexFor(&storage.KeyValueStore{}),
				testutil.WithKubeconfig(),
			)
			buf := &bytes.Buffer{}
			cmd := NewTestCmd(buf, tt.out)
			cmd.AllProjects = tt.inAllProjects
			err := tt.get.Run(t.Context(), apiClient, cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("keyValueStoreCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				for _, substr := range tt.wantContain {
					if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(substr)) {
						t.Errorf("keyValueStoreCmd.Run() error did not contain %q, err = %v", substr, err)
					}
				}
				return
			}

			for _, substr := range tt.wantContain {
				if !strings.Contains(buf.String(), substr) {
					t.Errorf("keyValueStoreCmd.Run() did not contain %q, out = %q", tt.wantContain, buf.String())
				}
			}
			if testutil.CountLines(buf.String()) != tt.wantLines {
				t.Errorf("expected the output to have %d lines, but found %d", tt.wantLines, testutil.CountLines(buf.String()))
				t.Log(buf.String())
			}
		})
	}
}

func kvsMem(mem string) *storage.KeyValueStoreMemorySize {
	return &storage.KeyValueStoreMemorySize{Quantity: resource.MustParse(mem)}
}
