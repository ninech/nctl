package update

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	networking "github.com/ninech/apis/networking/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/create"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestServiceConnection(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		update  serviceConnectionCmd
		want    networking.ServiceConnectionParameters
		wantErr bool
	}{
		{
			name: "addClusterOptions",
			update: serviceConnectionCmd{
				KubernetesClusterOptions: create.KubernetesClusterOptions{
					PodSelector: &create.LabelSelector{
						LabelSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"key1": "value1",
							},
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "key2",
									Operator: metav1.LabelSelectorOperator("In"),
									Values:   []string{"value1", "value2"},
								},
								{
									Key:      "key3",
									Operator: metav1.LabelSelectorOperator("Exists"),
								},
							},
						},
					},
					NamespaceSelector: &create.LabelSelector{
						LabelSelector: metav1.LabelSelector{
							MatchLabels: map[string]string{
								"key4": "value4",
							},
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "key5",
									Operator: metav1.LabelSelectorOperator("In"),
									Values:   []string{"value3", "value4"},
								},
								{
									Key:      "key6",
									Operator: metav1.LabelSelectorOperator("Exists"),
								},
							},
						},
					},
				},
			},
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.update.Name = "test-" + t.Name()

			apiClient, err := test.SetupClient()
			require.NoError(t, err)

			created := test.ServiceConnection(tt.update.Name, apiClient.Project)
			if err := apiClient.Create(ctx, created); err != nil {
				t.Fatalf("serviceconnection create error, got: %s", err)
			}
			if err := apiClient.Get(ctx, api.ObjectName(created), created); err != nil {
				t.Fatalf("expected serviceconnection to exist, got: %s", err)
			}

			updated := &networking.ServiceConnection{}
			if err := tt.update.Run(ctx, apiClient); (err != nil) != tt.wantErr {
				t.Errorf("serviceConnectionCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := apiClient.Get(ctx, api.ObjectName(created), updated); err != nil {
				t.Fatalf("expected serviceconnection to exist, got: %s", err)
			}

			if !cmp.Equal(updated.Spec.ForProvider, tt.want) {
				t.Fatalf("expected serviceConnection.Spec.ForProvider = %v, got: %v", updated.Spec.ForProvider, tt.want)
			}
		})
	}
}
