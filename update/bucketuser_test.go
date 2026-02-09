package update

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	meta "github.com/ninech/apis/meta/v1alpha1"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestBucketUser(t *testing.T) {
	t.Parallel()

	apiClient, err := test.SetupClient()
	if err != nil {
		t.Fatalf("setup client error, got: %s", err)
	}

	created := bucketUser("user", apiClient.Project, "nine-es34")
	if err := apiClient.Create(t.Context(), created); err != nil {
		t.Fatalf("bucketuser create error, got: %s", err)
	}
	if err := apiClient.Get(t.Context(), api.ObjectName(created), created); err != nil {
		t.Fatalf("expected bucketuser to exist, got: %s", err)
	}

	out := &bytes.Buffer{}
	cmd := bucketUserCmd{resourceCmd{Writer: format.NewWriter(out), Name: created.Name}, ptr.To(true)}
	updated := &storage.BucketUser{}
	if err := cmd.Run(t.Context(), apiClient); err != nil {
		t.Errorf("did not expect err got : %v", err)
	}
	if err := apiClient.Get(t.Context(), api.ObjectName(created), updated); err != nil {
		t.Fatalf("expected bucketuser to exist, got: %s", err)
	}

	if cmp.Equal(updated.Spec.ForProvider.ResetCredentials, created.Spec.ForProvider.ResetCredentials) {
		t.Fatalf("expected ResetCredentials field to differ, expected= %v, got: %v", updated.Spec.ForProvider.ResetCredentials, created.Spec.ForProvider.ResetCredentials)
	}

	if !strings.Contains(out.String(), "updated") {
		t.Errorf("expected output to contain 'updated', got %q", out.String())
	}
	if !strings.Contains(out.String(), created.Name) {
		t.Errorf("expected output to contain %q, got %q", created.Name, out.String())
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
