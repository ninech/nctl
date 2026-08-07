package update

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	observability "github.com/ninech/apis/observability/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
)

func TestGrafana(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		create observability.GrafanaParameters
		update grafanaCmd
		want   observability.GrafanaParameters
		// wantUnchanged expects the update to be a no-op, which succeeds
		// without writing the instance.
		wantUnchanged bool
	}{
		{
			name:          "simple",
			wantUnchanged: true,
		},
		{
			name:          "empty update",
			update:        grafanaCmd{},
			wantUnchanged: true,
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

			apiClient := test.SetupClient(t)

			created := test.Grafana(tt.update.Name, apiClient.Project)
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(t.Context(), created); err != nil {
				t.Fatalf("grafana create error, got: %s", err)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), created); err != nil {
				t.Fatalf("expected grafana to exist, got: %s", err)
			}

			updated := &observability.Grafana{}
			if err := tt.update.Run(t.Context(), apiClient); err != nil {
				t.Errorf("grafanaCmd.Run() error = %v", err)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), updated); err != nil {
				t.Fatalf("expected grafana to exist, got: %s", err)
			}

			if !cmp.Equal(updated.Spec.ForProvider, tt.want) {
				t.Fatalf("expected grafana.Spec.ForProvider = %v, got: %v", tt.want, updated.Spec.ForProvider)
			}

			wantOutput := "updated"
			if tt.wantUnchanged {
				wantOutput = "no changes made"
			}
			if !strings.Contains(out.String(), wantOutput) {
				t.Fatalf("expected output to contain %q, got: %s", wantOutput, out.String())
			}
			if !strings.Contains(out.String(), tt.update.Name) {
				t.Fatalf("expected output to contain %s, got: %s", tt.update.Name, out.String())
			}
		})
	}
}
