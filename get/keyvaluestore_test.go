package get

import (
	"bytes"
	"context"
	"strings"
	"testing"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestKeyValueStore(t *testing.T) {
	tests := []struct {
		name        string
		instances   map[string]storage.RedisParameters
		get         keyValueStoreCmd
		out         output
		wantContain []string
		wantErr     bool
	}{
		{"simple", map[string]storage.RedisParameters{}, keyValueStoreCmd{}, full, []string{"no Redis found"}, false},
		{
			"single",
			map[string]storage.RedisParameters{"test": {MemorySize: &storage.RedisMemorySize{Quantity: resource.MustParse("1G")}}},
			keyValueStoreCmd{},
			full,
			[]string{"1G"},
			false,
		},
		{
			"multiple",
			map[string]storage.RedisParameters{
				"test1": {MemorySize: &storage.RedisMemorySize{Quantity: resource.MustParse("1G")}},
				"test2": {MemorySize: &storage.RedisMemorySize{Quantity: resource.MustParse("2G")}},
				"test3": {MemorySize: &storage.RedisMemorySize{Quantity: resource.MustParse("3G")}},
			},
			keyValueStoreCmd{},
			full,
			[]string{"1G", "2G", "test3"},
			false,
		},
		{
			"name",
			map[string]storage.RedisParameters{
				"test1": {MemorySize: &storage.RedisMemorySize{Quantity: resource.MustParse("1G")}},
				"test2": {MemorySize: &storage.RedisMemorySize{Quantity: resource.MustParse("2G")}},
			},
			keyValueStoreCmd{Name: "test1"},
			full,
			[]string{"test1", "1G"},
			false,
		},
		{
			"password",
			map[string]storage.RedisParameters{
				"test1": {MemorySize: &storage.RedisMemorySize{Quantity: resource.MustParse("1G")}},
				"test2": {MemorySize: &storage.RedisMemorySize{Quantity: resource.MustParse("2G")}},
			},
			keyValueStoreCmd{Name: "test2", PrintToken: true},
			full,
			[]string{"test2-topsecret"},
			false,
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
				created := test.KeyValueStore(name, "default", "nine-es34")
				created.Spec.ForProvider = instance
				objects = append(objects, created)
				objects = append(objects, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      created.GetWriteConnectionSecretToReference().Name,
						Namespace: created.GetWriteConnectionSecretToReference().Namespace,
					},
					Data: map[string][]byte{"default": []byte(created.GetWriteConnectionSecretToReference().Name + "-topsecret")},
				})
			}

			client := fake.NewClientBuilder().
				WithScheme(scheme).
				WithIndex(&storage.Redis{}, "metadata.name", func(o client.Object) []string {
					return []string{o.GetName()}
				}).
				WithObjects(objects...).Build()
			apiClient := &api.Client{WithWatch: client, Project: "default"}
			ctx := context.Background()

			if err := tt.get.Run(ctx, apiClient, &Cmd{Output: tt.out}); (err != nil) != tt.wantErr {
				t.Errorf("keyValueStoreCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			for _, substr := range tt.wantContain {
				if !strings.Contains(buf.String(), substr) {
					t.Errorf("keyValueStoreCmd.Run() did not contain %q, out = %q", tt.wantContain, buf.String())
				}
			}
		})
	}
}
