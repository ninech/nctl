package application

import (
	"bytes"
	"strings"
	"testing"

	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	"github.com/ninech/nctl/internal/format"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestServicesFromMap(t *testing.T) {
	t.Parallel()

	kvsRef := TypedReference{}
	require.NoError(t, kvsRef.UnmarshalText([]byte("keyvaluestore/my-kvs")))

	mysqlRef := TypedReference{}
	require.NoError(t, mysqlRef.UnmarshalText([]byte("mysql/my-db")))

	tests := []struct {
		name      string
		services  ServiceMap
		namespace string
		want      apps.NamedServiceTargetList
	}{
		{
			name:     "nil map",
			services: nil,
			want:     nil,
		},
		{
			name:      "single service",
			services:  ServiceMap{"cache": kvsRef},
			namespace: "my-project",
			want: apps.NamedServiceTargetList{
				{
					Name: "cache",
					Target: meta.TypedReference{
						Reference: meta.Reference{Name: "my-kvs", Namespace: "my-project"},
						GroupKind: kvsRef.GroupKind,
					},
				},
			},
		},
		{
			name: "multiple services sorted",
			services: ServiceMap{
				"db":    mysqlRef,
				"cache": kvsRef,
			},
			namespace: "default",
			want: apps.NamedServiceTargetList{
				{
					Name: "cache",
					Target: meta.TypedReference{
						Reference: meta.Reference{Name: "my-kvs", Namespace: "default"},
						GroupKind: kvsRef.GroupKind,
					},
				},
				{
					Name: "db",
					Target: meta.TypedReference{
						Reference: meta.Reference{Name: "my-db", Namespace: "default"},
						GroupKind: mysqlRef.GroupKind,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := ServicesFromMap(tt.services, tt.namespace)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestTypedReference_UnmarshalText(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid keyvaluestore", "keyvaluestore/my-kvs", false},
		{"valid mysql", "mysql/my-db", false},
		{"empty", "", true},
		{"no slash", "keyvaluestore", true},
		{"missing name", "keyvaluestore/", true},
		{"invalid kind", "invalid/my-resource", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &TypedReference{}
			err := r.UnmarshalText([]byte(tt.input))
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, r.Name)
				require.NotEmpty(t, r.Kind)
			}
		})
	}
}

func TestUpdateServices(t *testing.T) {
	t.Parallel()

	kvsTarget := meta.TypedReference{
		Reference: meta.Reference{Name: "my-kvs", Namespace: "default"},
		GroupKind: metav1.GroupKind{Group: "storage.nine.ch", Kind: "KeyValueStore"},
	}
	mysqlTarget := meta.TypedReference{
		Reference: meta.Reference{Name: "my-db", Namespace: "default"},
		GroupKind: metav1.GroupKind{Group: "storage.nine.ch", Kind: "MySQL"},
	}
	newKvsTarget := meta.TypedReference{
		Reference: meta.Reference{Name: "new-kvs", Namespace: "default"},
		GroupKind: metav1.GroupKind{Group: "storage.nine.ch", Kind: "KeyValueStore"},
	}

	tests := []struct {
		name     string
		existing apps.NamedServiceTargetList
		toAdd    apps.NamedServiceTargetList
		toDelete []string
		want     apps.NamedServiceTargetList
		wantWarn bool
	}{
		{
			name:     "add to empty",
			existing: nil,
			toAdd:    apps.NamedServiceTargetList{{Name: "cache", Target: kvsTarget}},
			want:     apps.NamedServiceTargetList{{Name: "cache", Target: kvsTarget}},
		},
		{
			name:     "add new service",
			existing: apps.NamedServiceTargetList{{Name: "cache", Target: kvsTarget}},
			toAdd:    apps.NamedServiceTargetList{{Name: "db", Target: mysqlTarget}},
			want: apps.NamedServiceTargetList{
				{Name: "cache", Target: kvsTarget},
				{Name: "db", Target: mysqlTarget},
			},
		},
		{
			name:     "update existing service",
			existing: apps.NamedServiceTargetList{{Name: "cache", Target: kvsTarget}},
			toAdd:    apps.NamedServiceTargetList{{Name: "cache", Target: newKvsTarget}},
			want:     apps.NamedServiceTargetList{{Name: "cache", Target: newKvsTarget}},
		},
		{
			name:     "delete service",
			existing: apps.NamedServiceTargetList{{Name: "cache", Target: kvsTarget}, {Name: "db", Target: mysqlTarget}},
			toDelete: []string{"cache"},
			want:     apps.NamedServiceTargetList{{Name: "db", Target: mysqlTarget}},
		},
		{
			name:     "delete non-existent warns",
			existing: apps.NamedServiceTargetList{{Name: "cache", Target: kvsTarget}},
			toDelete: []string{"nonexistent"},
			want:     apps.NamedServiceTargetList{{Name: "cache", Target: kvsTarget}},
			wantWarn: true,
		},
		{
			name:     "add and delete",
			existing: apps.NamedServiceTargetList{{Name: "cache", Target: kvsTarget}},
			toAdd:    apps.NamedServiceTargetList{{Name: "db", Target: mysqlTarget}},
			toDelete: []string{"cache"},
			want:     apps.NamedServiceTargetList{{Name: "db", Target: mysqlTarget}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			buf := &bytes.Buffer{}
			w := format.NewWriter(buf)
			got := UpdateServices(tt.existing, tt.toAdd, tt.toDelete, w)
			require.Equal(t, tt.want, got)
			if tt.wantWarn {
				require.True(t, strings.Contains(buf.String(), "did not find"), "expected warning but got: %q", buf.String())
			}
		})
	}
}
