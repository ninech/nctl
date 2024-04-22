package get

import (
	"bytes"
	"context"
	"strings"
	"testing"

	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMySQL(t *testing.T) {
	tests := []struct {
		name      string
		instances map[string]storage.MySQLParameters
		get       mySQLCmd
		// out defines the output format and will bet set to "full" if
		// not given
		out         output
		wantContain []string
		wantErr     bool
	}{
		{
			name:        "simple",
			wantContain: []string{"no MySQLs found"},
		},
		{
			name:        "single",
			instances:   map[string]storage.MySQLParameters{"test": {MachineType: infra.MachineType("nine-standard-1")}},
			wantContain: []string{"nine-standard-1"},
		},
		{
			name: "multiple",
			instances: map[string]storage.MySQLParameters{
				"test1": {MachineType: infra.MachineType("nine-standard-1")},
				"test2": {MachineType: infra.MachineType("nine-standard-2")},
				"test3": {MachineType: infra.MachineType("nine-standard-4")},
			},
			wantContain: []string{"nine-standard-1", "nine-standard-2", "test3"},
		},
		{
			name: "get-by-name",
			instances: map[string]storage.MySQLParameters{
				"test1": {MachineType: infra.MachineType("nine-standard-1")},
				"test2": {MachineType: infra.MachineType("nine-standard-2")},
			},
			get:         mySQLCmd{Name: "test1"},
			wantContain: []string{"test1", "nine-standard-1"},
		},
		{
			name: "show-password",
			instances: map[string]storage.MySQLParameters{
				"test1": {MachineType: infra.MachineType("nine-standard-1")},
				"test2": {MachineType: infra.MachineType("nine-standard-2")},
			},
			get:         mySQLCmd{Name: "test2", PrintPassword: true},
			wantContain: []string{"test2-topsecret"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			tt.get.out = buf

			scheme, err := api.NewScheme()
			if err != nil {
				t.Fatal(err)
			}

			objects := []client.Object{}
			for name, instance := range tt.instances {
				created := test.MySQL(name, "default", "nine-es34")
				created.Spec.ForProvider = instance
				objects = append(objects, created, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      created.GetWriteConnectionSecretToReference().Name,
						Namespace: created.GetWriteConnectionSecretToReference().Namespace,
					},
					Data: map[string][]byte{storage.MySQLUser: []byte(created.GetWriteConnectionSecretToReference().Name + "-topsecret")},
				})
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithIndex(&storage.MySQL{}, "metadata.name", func(o client.Object) []string {
					return []string{o.GetName()}
				}).
				WithObjects(objects...).Build()
			apiClient := &api.Client{WithWatch: client, Project: "default"}
			ctx := context.Background()

			if tt.out == "" {
				tt.out = full
			}
			if err := tt.get.Run(ctx, apiClient, &Cmd{Output: tt.out}); (err != nil) != tt.wantErr {
				t.Errorf("mySQLCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			for _, substr := range tt.wantContain {
				if !strings.Contains(buf.String(), substr) {
					t.Errorf("mySQLCmd.Run() did not contain %q, out = %q", tt.wantContain, buf.String())
				}
			}
		})
	}
}
