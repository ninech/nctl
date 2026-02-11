package delete

import (
	"bytes"
	"strings"
	"testing"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
	"github.com/ninech/nctl/internal/test"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestProjectConfig(t *testing.T) {
	t.Parallel()
	project := "some-project"

	cfg := &apps.ProjectConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      project,
			Namespace: project,
		},
		Spec: apps.ProjectConfigSpec{},
	}

	out := &bytes.Buffer{}
	cmd := configCmd{
		Writer: format.NewWriter(out),
		Force:  true,
		Wait:   false,
	}

	apiClient := test.SetupClient(t,
		test.WithProjects(project),
		test.WithDefaultProject(project),
		test.WithObjects(cfg),
	)
	ctx := t.Context()

	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatalf("failed to run project configuration delete command: %v", err)
	}

	if !kerrors.IsNotFound(apiClient.Get(ctx, api.ObjectName(cfg), cfg)) {
		t.Fatal("expected project configuration to be deleted, but it still exists")
	}

	if !strings.Contains(out.String(), "deletion started") {
		t.Errorf("expected output to contain 'deletion started', got %q", out.String())
	}
	if !strings.Contains(out.String(), apps.ProjectConfigKind) {
		t.Errorf("expected output to contain kind %q, got %q", apps.ProjectConfigKind, out.String())
	}
}
