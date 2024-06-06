package create

import (
	"context"
	"reflect"
	"testing"
	"time"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestKeyValueStore(t *testing.T) {
	tests := []struct {
		name    string
		create  keyValueStoreCmd
		want    storage.KeyValueStoreParameters
		wantErr bool
	}{
		{"simple", keyValueStoreCmd{}, storage.KeyValueStoreParameters{}, false},
		{
			"memorySize",
			keyValueStoreCmd{MemorySize: "1G"},
			storage.KeyValueStoreParameters{MemorySize: &storage.KeyValueStoreMemorySize{Quantity: resource.MustParse("1G")}},
			false,
		},
		{
			"maxMemoryPolicy",
			keyValueStoreCmd{MaxMemoryPolicy: storage.KeyValueStoreMaxMemoryPolicy("noeviction")},
			storage.KeyValueStoreParameters{MaxMemoryPolicy: storage.KeyValueStoreMaxMemoryPolicy("noeviction")},
			false,
		},
		{
			"allowedCIDRs",
			keyValueStoreCmd{AllowedCidrs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
			storage.KeyValueStoreParameters{AllowedCIDRs: []meta.IPv4CIDR{meta.IPv4CIDR("0.0.0.0/0")}},
			false,
		},
		{
			"invalid",
			keyValueStoreCmd{MemorySize: "invalid"},
			storage.KeyValueStoreParameters{},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.create.Name = "test-" + t.Name()
			tt.create.Wait = false
			tt.create.WaitTimeout = time.Second

			scheme, err := api.NewScheme()
			if err != nil {
				t.Fatal(err)
			}
			client := fake.NewClientBuilder().WithScheme(scheme).Build()
			apiClient := &api.Client{WithWatch: client, Project: "default"}
			ctx := context.Background()

			if err := tt.create.Run(ctx, apiClient); (err != nil) != tt.wantErr {
				t.Errorf("keyValueStoreCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			created := &storage.KeyValueStore{ObjectMeta: metav1.ObjectMeta{Name: tt.create.Name, Namespace: apiClient.Project}}
			if err := apiClient.Get(ctx, api.ObjectName(created), created); (err != nil) != tt.wantErr {
				t.Fatalf("expected keyvaluestore to exist, got: %s", err)
			}
			if tt.wantErr {
				return
			}

			if !reflect.DeepEqual(created.Spec.ForProvider, tt.want) {
				t.Fatalf("expected KeyValueStore.Spec.ForProvider = %v, got: %v", created.Spec.ForProvider, tt.want)
			}
		})
	}
}
