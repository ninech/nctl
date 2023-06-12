package get

import (
	"bytes"
	"context"
	"testing"

	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestBuild(t *testing.T) {
	build := apps.Build{
		TypeMeta: metav1.TypeMeta{
			Kind:       apps.BuildKind,
			APIVersion: apps.BuildGroupVersionKind.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: apps.BuildSpec{},
	}
	build2 := build
	build2.Name = build2.Name + "-2"

	get := &Cmd{
		Output: full,
	}

	scheme, err := api.NewScheme()
	if err != nil {
		t.Fatal(err)
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&apps.Build{}, "metadata.name", func(o client.Object) []string {
			return []string{o.GetName()}
		}).
		WithObjects(&build, &build2).Build()
	apiClient := &api.Client{WithWatch: client, Project: "default"}
	ctx := context.Background()

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
