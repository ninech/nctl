package update

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	apps "github.com/ninech/apis/apps/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStaticEgress(t *testing.T) {
	t.Parallel()

	target := meta.LocalTypedReference{
		LocalReference: meta.LocalReference{Name: "my-app"},
		GroupKind:      metav1.GroupKind{Group: apps.Group, Kind: apps.ApplicationKind},
	}

	tests := []struct {
		name    string
		create  networking.StaticEgressParameters
		update  staticEgressCmd
		want    networking.StaticEgressParameters
		wantErr bool
	}{
		{
			name: "empty update",
			create: networking.StaticEgressParameters{
				Target: target,
			},
			update: staticEgressCmd{},
			want: networking.StaticEgressParameters{
				Target: target,
			},
		},
		{
			name: "enable disabled",
			create: networking.StaticEgressParameters{
				Target: target,
			},
			update: staticEgressCmd{Disabled: new(true)},
			want: networking.StaticEgressParameters{
				Disabled: true,
				Target:   target,
			},
		},
		{
			name: "disable disabled",
			create: networking.StaticEgressParameters{
				Disabled: true,
				Target:   target,
			},
			update: staticEgressCmd{Disabled: new(false)},
			want: networking.StaticEgressParameters{
				Target: target,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			out := &bytes.Buffer{}
			tt.update.Writer = format.NewWriter(out)
			tt.update.Name = "test-" + t.Name()

			apiClient := test.SetupClient(t)

			created := test.StaticEgress(tt.update.Name, apiClient.Project, "my-app")
			created.Spec.ForProvider = tt.create
			if err := apiClient.Create(t.Context(), created); err != nil {
				t.Fatalf("static egress create error, got: %s", err)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), created); err != nil {
				t.Fatalf("expected static egress to exist, got: %s", err)
			}

			updated := &networking.StaticEgress{}
			if err := tt.update.Run(t.Context(), apiClient); (err != nil) != tt.wantErr {
				t.Errorf("staticEgressCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err := apiClient.Get(t.Context(), api.ObjectName(created), updated); err != nil {
				t.Fatalf("expected static egress to exist, got: %s", err)
			}

			if !cmp.Equal(updated.Spec.ForProvider, tt.want) {
				t.Fatalf("expected staticEgress.Spec.ForProvider = %v, got: %v", tt.want, updated.Spec.ForProvider)
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
