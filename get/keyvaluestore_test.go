package get

import (
	"bytes"
	"context"
	"strings"
	"testing"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestKeyValueStore(t *testing.T) {
	ctx := context.Background()

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
					project: test.DefaultProject,
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
					project: test.DefaultProject,
					memSize: kvsMem("1G"),
				},
				{
					name:    "test2",
					project: test.DefaultProject,
					memSize: kvsMem("2G"),
				},
				{
					name:    "test3",
					project: test.DefaultProject,
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
					project: test.DefaultProject,
					memSize: kvsMem("1G"),
				},
				{
					name:    "test2",
					project: test.DefaultProject,
					memSize: kvsMem("2G"),
				},
			},
			get:         keyValueStoreCmd{resourceCmd: resourceCmd{Name: "test1"}},
			out:         full,
			wantContain: []string{"test1", "1G"},
			wantLines:   2, // header + result
		},
		{
			name: "multiple instances in multiple projects",
			instances: []kvsInstance{
				{
					name:    "test1",
					project: test.DefaultProject,
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
					project: test.DefaultProject,
					memSize: kvsMem("1G"),
				},
				{
					name:    "test2",
					project: test.DefaultProject,
					memSize: kvsMem("2G"),
				},
			},
			get:         keyValueStoreCmd{resourceCmd: resourceCmd{Name: "test2"}, PrintToken: true},
			out:         full,
			wantContain: []string{"test2-topsecret"},
			wantLines:   1, // print token does not print any header line
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objects := []client.Object{}
			for _, instance := range tt.instances {
				created := test.KeyValueStore(instance.name, instance.project, "nine-es34")
				created.Spec.ForProvider.MemorySize = instance.memSize
				objects = append(objects, created)
				objects = append(objects, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      created.GetWriteConnectionSecretToReference().Name,
						Namespace: created.GetWriteConnectionSecretToReference().Namespace,
					},
					Data: map[string][]byte{"default": []byte(created.GetWriteConnectionSecretToReference().Name + "-topsecret")},
				})
			}
			apiClient, err := test.SetupClient(
				test.WithProjectsFromResources(objects...),
				test.WithObjects(objects...),
				test.WithNameIndexFor(&storage.KeyValueStore{}),
				test.WithKubeconfig(t),
			)
			require.NoError(t, err)

			buf := &bytes.Buffer{}
			cmd := NewTestCmd(buf, tt.out)
			cmd.AllProjects = tt.inAllProjects
			err = tt.get.Run(ctx, apiClient, cmd)
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
			if test.CountLines(buf.String()) != tt.wantLines {
				t.Errorf("expected the output to have %d lines, but found %d", tt.wantLines, test.CountLines(buf.String()))
				t.Log(buf.String())
			}
		})
	}
}

func kvsMem(mem string) *storage.KeyValueStoreMemorySize {
	return &storage.KeyValueStoreMemorySize{Quantity: resource.MustParse(mem)}
}
