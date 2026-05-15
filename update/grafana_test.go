package update

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	observability "github.com/ninech/apis/observability/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestGrafana(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		create           observability.GrafanaParameters
		update           grafanaCmd
		want             observability.GrafanaParameters
		wantErr          bool
		interceptorFuncs *interceptor.Funcs
	}{
		{
			name: "simple",
		},
		{
			name:   "empty update",
			update: grafanaCmd{},
		},
		{
			name:    "no-flags",
			wantErr: true,
			interceptorFuncs: &interceptor.Funcs{
				Update: func(_ context.Context, _ client.WithWatch, _ client.Object, _ ...client.UpdateOption) error {
					return nil
				},
			},
		},
		{
			name:   "enable admin access",
			update: grafanaCmd{AdminAccess: new(true)},
			want:   observability.GrafanaParameters{EnableAdminAccess: true},
		},
		{
			name:   "disable admin access",
			create: observability.GrafanaParameters{EnableAdminAccess: true},
			update: grafanaCmd{AdminAccess: new(false)},
			want:   observability.GrafanaParameters{EnableAdminAccess: false},
		},
		{
			name:   "allow local users",
			update: grafanaCmd{LocalUsers: new(true)},
			want:   observability.GrafanaParameters{AllowLocalUsers: true},
		},
		{
			name:   "disallow local users",
			create: observability.GrafanaParameters{AllowLocalUsers: true},
			update: grafanaCmd{LocalUsers: new(false)},
			want:   observability.GrafanaParameters{AllowLocalUsers: false},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := &bytes.Buffer{}
			tt.update.Writer = format.NewWriter(out)
			tt.update.Name = "test-" + t.Name()

			opts := []test.ClientSetupOption{}
			if tt.interceptorFuncs != nil {
				opts = append(opts, test.WithInterceptorFuncs(*tt.interceptorFuncs))
			}
			apiClient := test.SetupClient(t, opts...)

			created := test.Grafana(tt.update.Name, apiClient.Project)
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(t.Context(), created); err != nil {
				t.Fatalf("grafana create error, got: %s", err)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), created); err != nil {
				t.Fatalf("expected grafana to exist, got: %s", err)
			}

			updated := &observability.Grafana{}
			if err := tt.update.Run(t.Context(), apiClient); (err != nil) != tt.wantErr {
				t.Errorf("grafanaCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), updated); err != nil {
				t.Fatalf("expected grafana to exist, got: %s", err)
			}

			if !cmp.Equal(updated.Spec.ForProvider, tt.want) {
				t.Fatalf("expected grafana.Spec.ForProvider = %v, got: %v", tt.want, updated.Spec.ForProvider)
			}

			if !tt.wantErr {
				if !strings.Contains(out.String(), "updated") {
					t.Fatalf("expected output to contain 'updated', got: %s", out.String())
				}
				if !strings.Contains(out.String(), tt.update.Name) {
					t.Fatalf("expected output to contain %s, got: %s", tt.update.Name, out.String())
				}
			}
		})
	}
}
