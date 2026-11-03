package api_test

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/testutil"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/rest"
)

func TestSetCreatedBy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		token       string
		annotations map[string]string
		want        map[string]string
	}{
		{
			name:  "user",
			token: testutil.FakeJWTToken,
			want:  map[string]string{api.CreatedByAnnotation: "jrocket@example.com"},
		},
		{
			name:        "keepsOtherAnnotations",
			token:       testutil.FakeJWTToken,
			annotations: map[string]string{"foo": "bar"},
			want:        map[string]string{"foo": "bar", api.CreatedByAnnotation: "jrocket@example.com"},
		},
		{
			name:        "keepsExistingCreator",
			token:       testutil.FakeJWTToken,
			annotations: map[string]string{api.CreatedByAnnotation: "someone@example.com"},
			want:        map[string]string{api.CreatedByAnnotation: "someone@example.com"},
		},
		{
			name:  "serviceAccount",
			token: testutil.ASAJWTToken,
			want:  map[string]string{api.CreatedByAnnotation: "example@1b6e8f9.serviceaccount.staging.nineapis.ch"},
		},
		{
			name:  "noEmail",
			token: tokenWithoutEmail(t),
			want:  map[string]string{api.CreatedByAnnotation: "d4ff929e"},
		},
		{
			name:  "invalidToken",
			token: "not-a-jwt",
			want:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &api.Client{Config: &rest.Config{BearerToken: tt.token}}
			obj := &storage.DatabaseRestore{}
			obj.SetAnnotations(tt.annotations)

			client.SetCreatedBy(t.Context(), obj)

			require.Equal(t, tt.want, obj.GetAnnotations())
		})
	}
}

func tokenWithoutEmail(t *testing.T) string {
	t.Helper()

	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "d4ff929e"}).
		SignedString([]byte("test"))
	require.NoError(t, err)

	return token
}
