package delete

import (
	"context"
	"testing"
	"time"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBucket(t *testing.T) {
	ctx := context.Background()
	cmd := bucketCmd{
		resourceCmd: resourceCmd{
			Name:        "test",
			Force:       true,
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	bu := bucket("test", test.DefaultProject, string(meta.LocationNineES34))
	apiClient, err := test.SetupClient(test.WithObjects(bu))
	require.NoError(t, err)

	if err := apiClient.Get(ctx, api.ObjectName(bu), bu); err != nil {
		t.Fatalf("expected bucket to exist, got: %s", err)
	}
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}
	err = apiClient.Get(ctx, api.ObjectName(bu), bu)
	if err == nil {
		t.Fatalf("expected bucket to be deleted, but exists")
	}
	if !errors.IsNotFound(err) {
		t.Fatalf("expected bucket to be deleted, got: %s", err.Error())
	}
}

func bucket(name, project, location string) *storage.Bucket {
	return &storage.Bucket{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		Spec: storage.BucketSpec{
			ForProvider: storage.BucketParameters{
				Location: meta.LocationName(location),
			},
		},
	}
}
