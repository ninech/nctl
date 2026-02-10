package get

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	infra "github.com/ninech/apis/infrastructure/v1alpha1"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestOpenSearch(t *testing.T) {
	ctx := context.Background()

	type openSearchInstance struct {
		name           string
		project        string
		machineType    infra.MachineType
		clusterType    storage.OpenSearchClusterType
		clusterHealth  storage.OpenSearchClusterHealth
		snapshotBucket string
	}

	tests := []struct {
		name      string
		instances []openSearchInstance
		get       openSearchCmd
		// out defines the output format and will bet set to "full" if
		// not given
		out           outputFormat
		want          string
		wantContain   []string
		wantLines     int
		inAllProjects bool
		wantErr       bool
	}{
		{
			name:        "simple",
			wantErr:     true,
			wantContain: []string{`no "OpenSearches" found`},
		},
		{
			name: "single instance in project",
			instances: []openSearchInstance{
				{
					name:        "test",
					project:     test.DefaultProject,
					machineType: infra.MachineTypeNineSearchS,
					clusterType: storage.OpenSearchClusterTypeSingle,
				},
			},
			wantContain: []string{"nine-search-s"},
			wantLines:   2, // header + result
		},
		{
			name: "multiple instances in one project",
			instances: []openSearchInstance{
				{
					name:        "test1",
					project:     test.DefaultProject,
					machineType: infra.MachineTypeNineSearchS,
				},
				{
					name:        "test2",
					project:     test.DefaultProject,
					machineType: infra.MachineTypeNineSearchM,
				},
			},
			wantContain: []string{"nine-search-s", "nine-search-m"},
			wantLines:   3, // header + result
		},
		{
			name: "multiple instances in multiple projects",
			instances: []openSearchInstance{
				{
					name:        "test1",
					project:     test.DefaultProject,
					machineType: machineType("nine-db-prod-s"),
				},
				{
					name:        "test2",
					project:     test.DefaultProject,
					machineType: machineType("nine-db-prod-m"),
				},
				{
					name:        "test3",
					project:     "testing",
					machineType: machineType("nine-db-prod-l"),
				},
			},
			wantContain:   []string{"test1", "test2", "test3"},
			inAllProjects: true,
			wantLines:     4, // header + result
		},
		{
			name: "get-by-name",
			instances: []openSearchInstance{
				{
					name:        "test1",
					project:     test.DefaultProject,
					machineType: infra.MachineTypeNineSearchS,
				},
				{
					name:        "test2",
					project:     test.DefaultProject,
					machineType: infra.MachineTypeNineSearchM,
				},
			},
			get:         openSearchCmd{resourceCmd: resourceCmd{Name: "test1"}},
			wantContain: []string{"test1", "nine-search-s"},
			wantLines:   2,
		},
		{
			name: "print snapshot bucket",
			instances: []openSearchInstance{
				{
					name:           "snapshot-instance",
					project:        test.DefaultProject,
					machineType:    infra.MachineTypeNineSearchS,
					snapshotBucket: "snapshot-instance-012345a",
				},
			},
			get:       openSearchCmd{resourceCmd: resourceCmd{Name: "snapshot-instance"}, PrintSnapshotBucket: true},
			want:      "https://nine-es34.objects.nineapis.ch/snapshot-instance-012345a",
			wantLines: 1,
		},
		{
			name: "show-password",
			instances: []openSearchInstance{
				{
					name:        "test1",
					project:     test.DefaultProject,
					machineType: infra.MachineTypeNineSearchL,
				},
				{
					name:        "test2",
					project:     test.DefaultProject,
					machineType: infra.MachineTypeNineSearchM,
				},
			},
			get:         openSearchCmd{resourceCmd: resourceCmd{Name: "test2"}, PrintPassword: true},
			wantContain: []string{"test2-topsecret"},
			wantLines:   1, // no header in this case
		},
		{
			name: "instance with green status",
			instances: []openSearchInstance{
				{
					name:        "healthy-instance",
					project:     test.DefaultProject,
					machineType: infra.MachineTypeNineSearchS,
				},
			},
			get:         openSearchCmd{resourceCmd: resourceCmd{Name: "healthy-instance"}},
			wantContain: []string{"green"},
			wantLines:   2,
		},
		{
			name: "instance with red status",
			instances: []openSearchInstance{
				{
					name:        "unhealthy-instance",
					project:     test.DefaultProject,
					machineType: infra.MachineTypeNineSearchS,
					clusterHealth: storage.OpenSearchClusterHealth{
						Indices: map[string]storage.OpenSearchClusterIndex{
							"unhealthy-instance": {
								Status: storage.OpenSearchHealthStatusRed,
							},
						},
					},
				},
			},
			get:         openSearchCmd{resourceCmd: resourceCmd{Name: "unhealthy-instance"}},
			wantContain: []string{"red"},
			wantLines:   2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}

			objects := []client.Object{}
			for _, instance := range tt.instances {
				created := test.OpenSearch(instance.name, instance.project, meta.LocationNineES34)
				created.Spec.ForProvider.MachineType = instance.machineType

				// Set cluster health status if provided
				if len(instance.clusterHealth.Indices) > 0 {
					created.Status.AtProvider.ClusterHealth = instance.clusterHealth
				}

				// Set snapshot bucket if provided
				if instance.snapshotBucket != "" {
					created.Status.AtProvider.SnapshotsBucket = meta.LocalReference{Name: instance.snapshotBucket}

					// Add the ObjectsBucket resource to the fake client
					bucket := &storage.Bucket{
						ObjectMeta: metav1.ObjectMeta{
							Name:      instance.snapshotBucket,
							Namespace: instance.project,
						},
						Spec: storage.BucketSpec{
							ForProvider: storage.BucketParameters{
								Location: meta.LocationNineCZ42,
							},
						},
						Status: storage.BucketStatus{
							AtProvider: storage.BucketObservation{
								PublicURL: strings.TrimSpace(fmt.Sprintf("https://%s.objects.nineapis.ch/%s", meta.LocationNineES34, instance.snapshotBucket)),
							},
						},
					}
					objects = append(objects, bucket)
				}

				objects = append(objects, created, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      created.GetWriteConnectionSecretToReference().Name,
						Namespace: created.GetWriteConnectionSecretToReference().Namespace,
					},
					Data: map[string][]byte{storage.OpenSearchUser: []byte(created.GetWriteConnectionSecretToReference().Name + "-topsecret")},
				})
			}
			apiClient, err := test.SetupClient(
				test.WithProjectsFromResources(objects...),
				test.WithObjects(objects...),
				test.WithNameIndexFor(&storage.OpenSearch{}),
				test.WithKubeconfig(t),
			)
			require.NoError(t, err)

			if tt.out == "" {
				tt.out = full
			}
			cmd := NewTestCmd(buf, tt.out)
			cmd.AllProjects = tt.inAllProjects
			err = tt.get.Run(ctx, apiClient, cmd)
			if (err != nil) != tt.wantErr {
				t.Errorf("openSearchCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				for _, substr := range tt.wantContain {
					if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(substr)) {
						t.Errorf("openSearchCmd.Run() error did not contain %q, err = %v", substr, err)
					}
				}
				return
			}

			for _, substr := range tt.wantContain {
				if !strings.Contains(buf.String(), substr) {
					t.Errorf("openSearchCmd.Run() did not contain %q, out = %q", tt.wantContain, buf.String())
				}
			}
			if test.CountLines(buf.String()) != tt.wantLines {
				t.Errorf("expected the output to have %d lines, but found %d", tt.wantLines, test.CountLines(buf.String()))
				t.Log(buf.String())
			}
		})
	}
}

func TestGetClusterHealth(t *testing.T) {
	cmd := &openSearchCmd{}

	tests := []struct {
		name          string
		clusterHealth storage.OpenSearchClusterHealth
		want          string
	}{
		{
			name: "no indices - should return green",
			clusterHealth: storage.OpenSearchClusterHealth{
				Indices: map[string]storage.OpenSearchClusterIndex{},
			},
			want: "green",
		},
		{
			name: "single green index",
			clusterHealth: storage.OpenSearchClusterHealth{
				Indices: map[string]storage.OpenSearchClusterIndex{
					"index1": {Status: storage.OpenSearchHealthStatusGreen},
				},
			},
			want: "green",
		},
		{
			name: "single yellow index",
			clusterHealth: storage.OpenSearchClusterHealth{
				Indices: map[string]storage.OpenSearchClusterIndex{
					"index1": {Status: storage.OpenSearchHealthStatusYellow},
				},
			},
			want: "yellow",
		},
		{
			name: "single red index",
			clusterHealth: storage.OpenSearchClusterHealth{
				Indices: map[string]storage.OpenSearchClusterIndex{
					"index1": {Status: storage.OpenSearchHealthStatusRed},
				},
			},
			want: "red",
		},
		{
			name: "multiple green indices",
			clusterHealth: storage.OpenSearchClusterHealth{
				Indices: map[string]storage.OpenSearchClusterIndex{
					"index1": {Status: storage.OpenSearchHealthStatusGreen},
					"index2": {Status: storage.OpenSearchHealthStatusGreen},
				},
			},
			want: "green",
		},
		{
			name: "green and yellow indices - should return yellow",
			clusterHealth: storage.OpenSearchClusterHealth{
				Indices: map[string]storage.OpenSearchClusterIndex{
					"index1": {Status: storage.OpenSearchHealthStatusGreen},
					"index2": {Status: storage.OpenSearchHealthStatusYellow},
				},
			},
			want: "yellow",
		},
		{
			name: "yellow and red indices - should return red (red has priority)",
			clusterHealth: storage.OpenSearchClusterHealth{
				Indices: map[string]storage.OpenSearchClusterIndex{
					"index1": {Status: storage.OpenSearchHealthStatusYellow},
					"index2": {Status: storage.OpenSearchHealthStatusRed},
				},
			},
			want: "red",
		},
		{
			name: "red and yellow indices (different order) - should return red",
			clusterHealth: storage.OpenSearchClusterHealth{
				Indices: map[string]storage.OpenSearchClusterIndex{
					"index1": {Status: storage.OpenSearchHealthStatusRed},
					"index2": {Status: storage.OpenSearchHealthStatusYellow},
				},
			},
			want: "red",
		},
		{
			name: "mixed green, yellow, and red - should return red",
			clusterHealth: storage.OpenSearchClusterHealth{
				Indices: map[string]storage.OpenSearchClusterIndex{
					"index1": {Status: storage.OpenSearchHealthStatusGreen},
					"index2": {Status: storage.OpenSearchHealthStatusYellow},
					"index3": {Status: storage.OpenSearchHealthStatusRed},
				},
			},
			want: "red",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(cmd.getClusterHealth(tt.clusterHealth))
			if got != tt.want {
				t.Errorf("getClusterHealth() = %v, want %v", got, tt.want)
			}
		})
	}
}
