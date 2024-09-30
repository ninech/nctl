package get

import (
	"bytes"
	"context"
	"testing"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuild(t *testing.T) {
	ctx := context.Background()
	build := apps.Build{
		TypeMeta: metav1.TypeMeta{
			Kind:       apps.BuildKind,
			APIVersion: apps.BuildGroupVersionKind.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: test.DefaultProject,
		},
		Spec: apps.BuildSpec{},
	}
	build2 := build
	build2.Name = build2.Name + "-2"

	get := &Cmd{
		Output: full,
	}

	apiClient, err := test.SetupClient(
		test.WithNameIndexFor(&apps.Build{}),
		test.WithObjects(&build, &build2),
	)
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	cmd := buildCmd{
		out: buf,
	}

	if err := cmd.Run(ctx, apiClient, get); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 3, test.CountLines(buf.String()))
	buf.Reset()

	cmd.Name = build.Name
	if err := cmd.Run(ctx, apiClient, get); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, test.CountLines(buf.String()))
	buf.Reset()

	get.Output = noHeader
	if err := cmd.Run(ctx, apiClient, get); err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, test.CountLines(buf.String()))
}
