package create

import (
	"context"
	"testing"
	"time"

	apps "github.com/ninech/apis/apps/v1alpha1"
	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	networking "github.com/ninech/apis/networking/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestServiceConnection(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name             string
		create           serviceConnectionCmd
		source           string
		destination      string
		want             networking.ServiceConnectionParameters
		wantErr          bool
		interceptorFuncs *interceptor.Funcs
	}{
		{
			name:    "noFlagsError",
			create:  serviceConnectionCmd{},
			want:    networking.ServiceConnectionParameters{},
			wantErr: true,
		},
		{
			name:        "default",
			create:      serviceConnectionCmd{},
			source:      "application/test-application",
			destination: "keyvaluestore/test-kvs",
			want: networking.ServiceConnectionParameters{
				Source: networking.Source{
					Reference: meta.TypedReference{
						Reference: meta.Reference{
							Name:      "test-application",
							Namespace: "default",
						},
						GroupKind: metav1.GroupKind{
							Group: apps.Group,
							Kind:  apps.ApplicationKind,
						},
					},
				},
				Destination: meta.TypedReference{
					Reference: meta.Reference{
						Name:      "test-kvs",
						Namespace: "default",
					},
					GroupKind: metav1.GroupKind{
						Group: storage.Group,
						Kind:  storage.KeyValueStoreKind,
					},
				},
			},
		},
		{
			name: "withNamespaces",
			create: serviceConnectionCmd{
				SourceNamespace: "application-ns",
			},
			source:      "application/test-application",
			destination: "keyvaluestore/test-kvs",
			want: networking.ServiceConnectionParameters{
				Source: networking.Source{
					Reference: meta.TypedReference{
						Reference: meta.Reference{
							Name:      "test-application",
							Namespace: "application-ns",
						},
						GroupKind: metav1.GroupKind{
							Group: apps.Group,
							Kind:  apps.ApplicationKind,
						},
					},
				},
				Destination: meta.TypedReference{
					Reference: meta.Reference{
						Name:      "test-kvs",
						Namespace: "default",
					},
					GroupKind: metav1.GroupKind{
						Group: storage.Group,
						Kind:  storage.KeyValueStoreKind,
					},
				},
			},
		},
		{
			name: "withClusterOptions",
			create: serviceConnectionCmd{
				KubernetesClusterOptions: KubernetesClusterOptions{
					PodSelector: &LabelSelector{
						LabelSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"key1": "value1",
							},
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "key2",
									Operator: "In",
									Values:   []string{"value1", "value2"},
								},
								{
									Key:      "key3",
									Operator: "Exists",
								},
							},
						},
					},
					NamespaceSelector: &LabelSelector{
						LabelSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"key4": "value4",
							},
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "key5",
									Operator: "In",
									Values:   []string{"value3", "value4"},
								},
								{
									Key:      "key6",
									Operator: "Exists",
								},
							},
						},
					},
				},
			},
			source:      "kubernetescluster/test-cluster",
			destination: "keyvaluestore/test-kvs",
			want: networking.ServiceConnectionParameters{
				Source: networking.Source{
					Reference: meta.TypedReference{
						Reference: meta.Reference{
							Name:      "test-cluster",
							Namespace: "default",
						},
						GroupKind: metav1.GroupKind{
							Group: infra.Group,
							Kind:  infra.KubernetesClusterKind,
						},
					},
					KubernetesClusterOptions: &networking.KubernetesClusterOptions{
						PodSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"key1": "value1",
							},
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "key2",
									Operator: "In",
									Values:   []string{"value1", "value2"},
								},
								{
									Key:      "key3",
									Operator: "Exists",
								},
							},
						},
						NamespaceSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"key4": "value4",
							},
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "key5",
									Operator: "In",
									Values:   []string{"value3", "value4"},
								},
								{
									Key:      "key6",
									Operator: "Exists",
								},
							},
						},
					},
				},
				Destination: meta.TypedReference{
					Reference: meta.Reference{
						Name:      "test-kvs",
						Namespace: "default",
					},
					GroupKind: metav1.GroupKind{
						Group: storage.Group,
						Kind:  storage.KeyValueStoreKind,
					},
				},
			},
		},
		{
			name:        "invalidSource",
			create:      serviceConnectionCmd{},
			source:      "invalidgroup/invalidkind",
			destination: "keyvaluestore/test-kvs",
			want:        networking.ServiceConnectionParameters{},
			wantErr:     true,
		},
		{
			name:        "invalidDestination",
			create:      serviceConnectionCmd{},
			source:      "kubernetescluster/test-cluster",
			destination: "invalidgroup/invalidkind",
			want:        networking.ServiceConnectionParameters{},
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := require.New(t)

			tt.create.Name = "test-" + t.Name()
			tt.create.Wait = false
			tt.create.WaitTimeout = time.Second

			opts := []test.ClientSetupOption{}
			if tt.interceptorFuncs != nil {
				opts = append(opts, test.WithInterceptorFuncs(*tt.interceptorFuncs))
			}
			apiClient, err := test.SetupClient(opts...)
			is.NoError(err)

			if err := tt.create.Source.UnmarshalText([]byte(tt.source)); err != nil {
				if tt.wantErr {
					return
				}
				t.Errorf("source.UnmarshalText() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err := tt.create.Destination.UnmarshalText([]byte(tt.destination)); err != nil {
				if tt.wantErr {
					return
				}
				t.Errorf("destination.UnmarshalText() error = %v, wantErr %v", err, tt.wantErr)
			}

			if err := tt.create.Run(ctx, apiClient); (err != nil) != tt.wantErr {
				t.Errorf("serviceConnectionCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			created := &networking.ServiceConnection{ObjectMeta: metav1.ObjectMeta{Name: tt.create.Name, Namespace: apiClient.Project}}
			if err := apiClient.Get(ctx, api.ObjectName(created), created); (err != nil) != tt.wantErr {
				t.Fatalf("expected serviceconnection to exist, got: %s", err)
			}
			if tt.wantErr {
				return
			}

			is.Equal(tt.want, created.Spec.ForProvider)
		})
	}
}

func TestLabelSelector_UnmarshalText(t *testing.T) {
	is := assert.New(t)

	tests := []struct {
		name    string
		arg     string
		want    metav1.LabelSelector
		wantErr bool
	}{
		{"none", "", metav1.LabelSelector{MatchLabels: nil, MatchExpressions: nil}, false},
		{"simple", "key1=value1", metav1.LabelSelector{MatchLabels: map[string]string{"key1": "value1"}, MatchExpressions: []metav1.LabelSelectorRequirement{}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ls := &LabelSelector{}
			err := ls.UnmarshalText([]byte(tt.arg))
			if err == nil && !tt.wantErr {
				is.Equal(tt.want, ls.LabelSelector)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("LabelSelector.UnmarshalText() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTypedReference_UnmarshalText(t *testing.T) {
	is := assert.New(t)

	tests := []struct {
		name    string
		arg     string
		want    meta.TypedReference
		wantErr bool
	}{
		{"none", "", meta.TypedReference{}, true},
		{"kvs", "keyvaluestore/prod", meta.TypedReference{Reference: meta.Reference{Name: "prod"}, GroupKind: metav1.GroupKind{Group: storage.Group, Kind: storage.KeyValueStoreKind}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := &TypedReference{}
			err := r.UnmarshalText([]byte(tt.arg))
			if err == nil && !tt.wantErr {
				is.Equal(tt.want, r.TypedReference)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("TypedReference.UnmarshalText() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
