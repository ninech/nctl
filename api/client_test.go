package api_test

import (
	"testing"

	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestClient_ConnectionSecretData(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	const project = "default"
	postgres := test.Postgres("pg", project, "nine-es34")
	data := map[string][]byte{"user": []byte("secret")}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      postgres.GetWriteConnectionSecretToReference().Name,
			Namespace: postgres.GetWriteConnectionSecretToReference().Namespace,
		},
		Data: data,
	}

	client := test.SetupClient(
		t,
		test.WithProjects(project),
		test.WithObjects(postgres, secret),
	)

	got, err := client.ConnectionSecretData(t.Context(), postgres)
	is.NoError(err)
	is.Equal(data, got)

	missing := test.Postgres("missing", project, "nine-es34")
	_, err = client.ConnectionSecretData(t.Context(), missing)
	is.Error(err)

	noRef := test.Postgres("noref", project, "nine-es34")
	noRef.Spec.WriteConnectionSecretToReference = nil
	_, err = client.ConnectionSecretData(t.Context(), noRef)
	is.Error(err)
}
