package create

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	apps "github.com/ninech/apis/apps/v1alpha1"
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestStaticEgress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		create           staticEgressCmd
		want             networking.StaticEgressParameters
		wantErr          bool
		interceptorFuncs *interceptor.Funcs
	}{
		{
			name:   "simple",
			create: staticEgressCmd{Application: "my-app"},
			want: networking.StaticEgressParameters{
				Target: meta.LocalTypedReference{
					LocalReference: meta.LocalReference{Name: "my-app"},
					GroupKind:      metav1.GroupKind{Group: apps.Group, Kind: apps.ApplicationKind},
				},
			},
		},
		{
			name:   "with disabled",
			create: staticEgressCmd{Application: "my-app", Disabled: true},
			want: networking.StaticEgressParameters{
				Disabled: true,
				Target: meta.LocalTypedReference{
					LocalReference: meta.LocalReference{Name: "my-app"},
					GroupKind:      metav1.GroupKind{Group: apps.Group, Kind: apps.ApplicationKind},
				},
			},
		},
		{
			name:   "cluster target",
			create: staticEgressCmd{Cluster: "my-cluster"},
			want: networking.StaticEgressParameters{
				Target: meta.LocalTypedReference{
					LocalReference: meta.LocalReference{Name: "my-cluster"},
					GroupKind:      metav1.GroupKind{Group: infrastructure.Group, Kind: infrastructure.KubernetesClusterKind},
				},
			},
		},
		{
			name:   "cluster target with disabled",
			create: staticEgressCmd{Cluster: "my-cluster", Disabled: true},
			want: networking.StaticEgressParameters{
				Disabled: true,
				Target: meta.LocalTypedReference{
					LocalReference: meta.LocalReference{Name: "my-cluster"},
					GroupKind:      metav1.GroupKind{Group: infrastructure.Group, Kind: infrastructure.KubernetesClusterKind},
				},
			},
		},
		{
			name:    "no target specified",
			create:  staticEgressCmd{},
			wantErr: true,
		},
		{
			name:    "error on creation",
			create:  staticEgressCmd{Application: "my-app"},
			wantErr: true,
			interceptorFuncs: &interceptor.Funcs{
				Create: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.CreateOption) error {
					return errors.New("error on creation")
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			tt.create.Name = "test-" + t.Name()
			tt.create.Wait = false
			tt.create.WaitTimeout = time.Second

			opts := []test.ClientSetupOption{}
			if tt.interceptorFuncs != nil {
				opts = append(opts, test.WithInterceptorFuncs(*tt.interceptorFuncs))
			}
			apiClient := test.SetupClient(t, opts...)

			if err := tt.create.Run(t.Context(), apiClient, &Cmd{}); (err != nil) != tt.wantErr {
				t.Errorf("staticEgressCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			created := &networking.StaticEgress{ObjectMeta: metav1.ObjectMeta{Name: tt.create.Name, Namespace: apiClient.Project}}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), created); (err != nil) != tt.wantErr {
				t.Fatalf("expected static egress to exist, got: %s", err)
			}
			if tt.wantErr {
				return
			}

			is.True(cmp.Equal(tt.want, created.Spec.ForProvider))
		})
	}
}
