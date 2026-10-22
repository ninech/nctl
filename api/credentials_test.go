package api_test

import (
	"context"
	"testing"
	"time"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/testutil"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestClient_RotateCredentials(t *testing.T) {
	t.Parallel()

	const project = "default"

	for name, tc := range map[string]struct {
		// withoutSecretRef removes the connection secret reference.
		withoutSecretRef bool
		// withoutSecret does not create the connection secret.
		withoutSecret  bool
		wantErr        string
		wantOld        types.UID
		wantAnnotation bool
	}{
		"deletes the secret and annotates the resource": {
			wantOld:        "old",
			wantAnnotation: true,
		},
		"annotates the resource if the secret is missing": {
			withoutSecret:  true,
			wantAnnotation: true,
		},
		"fails without a connection secret reference": {
			withoutSecretRef: true,
			wantErr:          "has no connection secret",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			postgres := testutil.Postgres("pg", project, "nine-es34")
			secret := connectionSecret(postgres.GetWriteConnectionSecretToReference().Name, project)
			objects := []runtimeclient.Object{postgres}
			if !tc.withoutSecret {
				objects = append(objects, secret)
			}
			if tc.withoutSecretRef {
				postgres.SetWriteConnectionSecretToReference(nil)
			}

			// secretAtPatch is the result of getting the secret
			// when the resource gets annotated.
			var secretAtPatch error
			client := testutil.SetupClient(t,
				testutil.WithProjects(project),
				testutil.WithObjects(objects...),
				testutil.WithInterceptorFuncs(interceptor.Funcs{
					Patch: func(ctx context.Context, c runtimeclient.WithWatch, obj runtimeclient.Object, patch runtimeclient.Patch, opts ...runtimeclient.PatchOption) error {
						secretAtPatch = c.Get(ctx, api.ObjectName(secret), &corev1.Secret{})
						return c.Patch(ctx, obj, patch, opts...)
					},
				}),
			)

			old, err := client.RotateCredentials(t.Context(), postgres)
			if tc.wantErr != "" {
				is.ErrorContains(err, tc.wantErr)
			} else {
				is.NoError(err)
			}
			is.Equal(tc.wantOld, old)

			if !tc.withoutSecret {
				getErr := client.Get(t.Context(), api.ObjectName(secret), &corev1.Secret{})
				if tc.wantErr == "" {
					is.True(kerrors.IsNotFound(getErr), "expected the secret to be deleted, got %v", getErr)
				} else {
					is.NoError(getErr)
				}
			}

			stored := postgres.DeepCopy()
			is.NoError(client.Get(t.Context(), api.ObjectName(postgres), stored))
			rotated, annotated := stored.GetAnnotations()[api.RotateCredentialsAnnotation]
			if !tc.wantAnnotation {
				is.False(annotated, "expected no %s annotation", api.RotateCredentialsAnnotation)
				return
			}
			is.True(annotated, "expected the %s annotation", api.RotateCredentialsAnnotation)
			_, err = time.Parse(time.RFC3339, rotated)
			is.NoError(err)
			// the reconcile triggered by the annotation must not find the old secret.
			is.True(kerrors.IsNotFound(secretAtPatch), "expected the secret to be deleted before the annotation, got %v", secretAtPatch)
		})
	}
}

func connectionSecret(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		// the fake client does not generate UIDs,
		// which the deletion precondition relies on.
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, UID: "old"},
		Data:       map[string][]byte{"password": []byte("old")},
	}
}
