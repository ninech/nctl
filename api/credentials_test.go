package api_test

import (
	"testing"
	"time"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/testutil"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestClient_RotateCredentials(t *testing.T) {
	t.Parallel()

	const project = "default"

	for name, tc := range map[string]struct {
		// withoutSecret does not create the connection secret.
		withoutSecret bool
		// annotatedNow sets the annotation to the current time already,
		// like a reset within the same second.
		annotatedNow bool
	}{
		"annotates the resource and leaves the secret alone": {},
		"annotates the resource if the secret is missing": {
			withoutSecret: true,
		},
		"uses a new value within the same second": {
			annotatedNow: true,
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
			var previous string
			if tc.annotatedNow {
				previous = time.Now().UTC().Format(time.RFC3339)
				postgres.SetAnnotations(map[string]string{api.RotateCredentialsAnnotation: previous})
			}
			client := testutil.SetupClient(t,
				testutil.WithProjects(project),
				testutil.WithObjects(objects...),
			)

			token, err := client.RotateCredentials(t.Context(), postgres)
			is.NoError(err)
			if !tc.withoutSecret {
				stored := &corev1.Secret{}
				is.NoError(client.Get(t.Context(), api.ObjectName(secret), stored))
				is.Equal(secret.Data, stored.Data)
			}

			stored := postgres.DeepCopy()
			is.NoError(client.Get(t.Context(), api.ObjectName(postgres), stored))
			annotation, annotated := stored.GetAnnotations()[api.RotateCredentialsAnnotation]
			is.True(annotated, "expected the %s annotation", api.RotateCredentialsAnnotation)
			is.Equal(token, annotation)
			is.NotEqual(previous, token)
			_, err = time.Parse(time.RFC3339, token)
			is.NoError(err)
		})
	}
}

func connectionSecret(name, namespace string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Data:       map[string][]byte{"password": []byte("old")},
	}
}
