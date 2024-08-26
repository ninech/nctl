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

func TestPostgres(t *testing.T) {
	tests := []struct {
		name      string
		instances map[string]storage.PostgresParameters
		get       postgresCmd
		// out defines the output format and will bet set to "full" if
		// not given
		out         output
		wantContain []string
		wantErr     bool
	}{
		{
			name:        "simple",
			wantContain: []string{"no Postgres found"},
		},
		{
			name:        "single",
			instances:   map[string]storage.PostgresParameters{"test": {MachineType: infra.MachineType("nine-db-prod-s")}},
			wantContain: []string{"nine-db-prod-s"},
		},
		{
			name: "multiple",
			instances: map[string]storage.PostgresParameters{
				"test1": {MachineType: infra.MachineType("nine-db-prod-s")},
				"test2": {MachineType: infra.MachineType("nine-db-prod-m")},
				"test3": {MachineType: infra.MachineType("nine-db-prod-l")},
			},
			wantContain: []string{"nine-db-prod-s", "nine-db-prod-m", "test3"},
		},
		{
			name: "get-by-name",
			instances: map[string]storage.PostgresParameters{
				"test1": {MachineType: infra.MachineType("nine-db-prod-s")},
				"test2": {MachineType: infra.MachineType("nine-db-prod-m")},
			},
			get:         postgresCmd{resourceCmd: resourceCmd{Name: "test1"}},
			wantContain: []string{"test1", "nine-db-prod-s"},
		},
		{
			name: "show-password",
			instances: map[string]storage.PostgresParameters{
				"test1": {MachineType: infra.MachineType("nine-db-prod-s")},
				"test2": {MachineType: infra.MachineType("nine-db-prod-m")},
			},
			get:         postgresCmd{resourceCmd: resourceCmd{Name: "test2"}, PrintPassword: true},
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
				created := test.Postgres(name, "default", "nine-es34")
				created.Spec.ForProvider = instance
				objects = append(objects, created, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      created.GetWriteConnectionSecretToReference().Name,
						Namespace: created.GetWriteConnectionSecretToReference().Namespace,
					},
					Data: map[string][]byte{storage.PostgresUser: []byte(created.GetWriteConnectionSecretToReference().Name + "-topsecret")},
				})
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithIndex(&storage.Postgres{}, "metadata.name", func(o client.Object) []string {
					return []string{o.GetName()}
				}).
				WithObjects(objects...).Build()
			apiClient := &api.Client{WithWatch: client, Project: "default"}
			ctx := context.Background()

			if tt.out == "" {
				tt.out = full
			}
			if err := tt.get.Run(ctx, apiClient, &Cmd{Output: tt.out}); (err != nil) != tt.wantErr {
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
		})
	}
}
