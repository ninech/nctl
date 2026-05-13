package update

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	apps "github.com/ninech/apis/apps/v1alpha1"
	infrastructure "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	networking "github.com/ninech/apis/networking/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestStaticEgress(t *testing.T) {
	t.Parallel()

	appTarget := meta.LocalTypedReference{
		LocalReference: meta.LocalReference{Name: "my-app"},
		GroupKind:      metav1.GroupKind{Group: apps.Group, Kind: apps.ApplicationKind},
	}
	clusterTarget := meta.LocalTypedReference{
		LocalReference: meta.LocalReference{Name: "my-cluster"},
		GroupKind:      metav1.GroupKind{Group: infrastructure.Group, Kind: infrastructure.KubernetesClusterKind},
	}

	tests := []struct {
		name       string
		create     networking.StaticEgressParameters
		update     staticEgressCmd
		want       networking.StaticEgressParameters
		targetName string
		wantErr    bool
	}{
		{
			name: "empty update",
			create: networking.StaticEgressParameters{
				Target: appTarget,
			},
			update:     staticEgressCmd{},
			targetName: "my-app",
			want: networking.StaticEgressParameters{
				Target: appTarget,
			},
		},
		{
			name: "enable disabled",
			create: networking.StaticEgressParameters{
				Target: appTarget,
			},
			update:     staticEgressCmd{Disabled: new(true)},
			targetName: "my-app",
			want: networking.StaticEgressParameters{
				Disabled: true,
				Target:   appTarget,
			},
		},
		{
			name: "disable disabled",
			create: networking.StaticEgressParameters{
				Disabled: true,
				Target:   appTarget,
			},
			update:     staticEgressCmd{Disabled: new(false)},
			targetName: "my-app",
			want: networking.StaticEgressParameters{
				Target: appTarget,
			},
		},
		{
			name: "cluster target enable disabled",
			create: networking.StaticEgressParameters{
				Target: clusterTarget,
			},
			update:     staticEgressCmd{Disabled: new(true)},
			targetName: "my-cluster",
			want: networking.StaticEgressParameters{
				Disabled: true,
				Target:   clusterTarget,
			},
		},
		{
			name: "cluster target disable disabled",
			create: networking.StaticEgressParameters{
				Disabled: true,
				Target:   clusterTarget,
			},
			update:     staticEgressCmd{Disabled: new(false)},
			targetName: "my-cluster",
			want: networking.StaticEgressParameters{
				Target: clusterTarget,
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

			targetName := tt.targetName
			if targetName == "" {
				targetName = "my-app"
			}
			var created *networking.StaticEgress
			if tt.create.Target.Kind == infrastructure.KubernetesClusterKind {
				created = test.StaticEgressForCluster(tt.update.Name, apiClient.Project, targetName)
			} else {
				created = test.StaticEgress(tt.update.Name, apiClient.Project, targetName)
			}
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
