package delete

import (
	"bytes"
	"strings"
	"testing"
	"time"

	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBucketUser(t *testing.T) {
	t.Parallel()
	out := &bytes.Buffer{}
	cmd := bucketUserCmd{
		resourceCmd: resourceCmd{
			Writer:      format.NewWriter(out),
			Name:        "test",
			Force:       true,
			Wait:        false,
			WaitTimeout: time.Second,
		},
	}

	bu := bucketUser("test", test.DefaultProject, "nine-es34")
	apiClient := test.SetupClient(t, test.WithObjects(bu))

	ctx := t.Context()
	if err := apiClient.Get(ctx, api.ObjectName(bu), bu); err != nil {
		t.Fatalf("expected bucketuser to exist before deletion, got error: %v", err)
	}
	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatalf("failed to run bucketuser delete command: %v", err)
	}
	err := apiClient.Get(ctx, api.ObjectName(bu), bu)
	if err == nil {
		t.Fatal("expected bucketuser to be deleted, but it still exists")
	}
	if !kerrors.IsNotFound(err) {
		t.Fatalf("expected bucketuser to be deleted (NotFound), but got error: %v", err)
	}

	if !strings.Contains(out.String(), "deletion started") {
		t.Errorf("expected output to contain 'deletion started', got %q", out.String())
	}
	if !strings.Contains(out.String(), cmd.Name) {
		t.Errorf("expected output to contain bucketuser name %q, got %q", cmd.Name, out.String())
	}
}

func bucketUser(name, project, location string) *storage.BucketUser {
	return &storage.BucketUser{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: project,
		},
		Spec: storage.BucketUserSpec{
			ForProvider: storage.BucketUserParameters{
				Location: meta.LocationName(location),
			},
		},
	}
}
